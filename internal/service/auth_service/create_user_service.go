package auth_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// CreateUser orchestre l'inscription : règles métier, génération des modèles BDD,
// écriture asynchrone (Write-Behind) et upload de l'avatar.
func CreateUser(ctx context.Context, input auth_models.SignUpInput, ipAddress string) (auth_models.SignUpResponse, error) {

	// 1. RÈGLES MÉTIER ET VÉRIFICATIONS D'UNICITÉ (BDD)
	// ---------------------------------------------------------
	if service.IsUnique(ctx, redis.EntityUser, "username", input.Username) == 0 {
		return auth_models.SignUpResponse{}, nubo_error.NewConflict("USERNAME_TAKEN", "Ce nom d'utilisateur est déjà pris.", nil)
	}
	if service.IsUnique(ctx, redis.EntityUser, "email", input.Email) == 0 {
		return auth_models.SignUpResponse{}, nubo_error.NewConflict("EMAIL_TAKEN", "Cet email est déjà utilisé.", nil)
	}
	if service.IsUnique(ctx, redis.EntityUser, "phone", input.Phone) == 0 {
		return auth_models.SignUpResponse{}, nubo_error.NewConflict("PHONE_TAKEN", "Ce numéro de téléphone est déjà utilisé.", nil)
	}

	parsedBirthdate, err := time.Parse("02012006", input.Birthdate)
	if err != nil {
		return auth_models.SignUpResponse{}, nubo_error.NewBadRequest("INVALID_DATE", "Le format de la date de naissance est invalide.", err)
	}

	age := time.Since(parsedBirthdate).Hours() / 24 / 365
	if age < 13 {
		return auth_models.SignUpResponse{}, nubo_error.NewBadRequest("AGE_UNDER_13", "Vous devez avoir au moins 13 ans.", nil)
	}
	if age > 120 {
		return auth_models.SignUpResponse{}, nubo_error.NewBadRequest("AGE_OVER_120", "Date de naissance invalide.", nil)
	}

	if input.Gender < 0 || input.Gender > 2 {
		return auth_models.SignUpResponse{}, nubo_error.NewBadRequest("INVALID_GENDER", "Genre invalide.", nil)
	}

	// 2. GÉNÉRATION DES DONNÉES (La "Vérité" absolue)
	// ---------------------------------------------------------
	now := time.Now().UTC()
	userID := pkg.GenerateID()
	sessionID := pkg.GenerateID()

	logger.Log.Info().Int64("user_id", userID).Msg("Création d'un nouvel utilisateur")

	// === ACTIVATION DU MÉDIA (Out-of-Band) ===
	if input.ProfilePictureID > 0 {
		if errAct := media_service.ActivateMediaBatch(ctx, []int64{input.ProfilePictureID}, userID); errAct != nil {
			return auth_models.SignUpResponse{}, nubo_error.NewBadRequest("AVATAR_ACTIVATION_FAILED", "Impossible de valider la photo de profil.", errAct)
		}
	}

	// A. Hydratation du Payload Utilisateur
	req := auth_models.UserPayload{
		ID:               userID,
		Username:         input.Username,
		Email:            input.Email,
		EmailVerified:    false,
		Phone:            input.Phone,
		PhoneVerified:    false,
		PasswordHash:     input.PasswordHash,
		FirstName:        pkg.CleanStr(input.FirstName),
		LastName:         pkg.CleanStr(input.LastName),
		Birthdate:        domain.TimeToMillis(parsedBirthdate),
		Sex:              input.Gender,
		Bio:              pkg.CleanStr(input.Bio),
		ProfilePictureID: input.ProfilePictureID,
		Grade:            0,
		Location:         pkg.CleanStr(input.Location),
		School:           pkg.CleanStr(input.School),
		Work:             pkg.CleanStr(input.Work),
		Badges:           []string{},
		Desactivated:     false,
		Banned:           false,
		CreatedAt:        domain.TimeToMillis(now),
		UpdatedAt:        domain.TimeToMillis(now),
	}

	// B. Hydratation du Payload Session
	sessions := auth_models.SessionsPayload{
		ID:                     sessionID,
		UserID:                 userID,
		FirebaseInstallationID: input.FirebaseInstallationID,
		DeviceInfo:             input.DeviceInfo,
		IPHistory:              []string{ipAddress},
		CreatedAt:              domain.TimeToMillis(now),
		ExpiresAt:              domain.TimeToMillis(now.Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)),
		ToleranceTime:          domain.TimeToMillis(now.Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second)),
	}

	// Génération des Tokens
	sessions.MasterToken, err = pkg.GenerateToken(req.ID, sessions.FirebaseInstallationID, variables.MasterTokenExpirationSeconds)
	if err != nil {
		return auth_models.SignUpResponse{}, nubo_error.NewInternal(err)
	}
	sessions.CurrentSecret = security.DeriveNextSecret(sessions.FirebaseInstallationID, sessions.MasterToken, sessions.MasterToken, sessions.FirebaseInstallationID)
	sessions.LastSecret = sessions.FirebaseInstallationID

	newJWT, err := pkg.GenerateToken(req.ID, sessions.FirebaseInstallationID, variables.JWTExpirationSeconds)
	if err != nil {
		return auth_models.SignUpResponse{}, nubo_error.NewInternal(err)
	}

	// C. Hydratation du Payload UserSettings
	// L'application envoie toujours l'objet Privacy et Notifications en entier (identités complètes)
	settingsID := pkg.GenerateID()
	settings := user_settings_models.UserSettingsPayload{
		ID:                 settingsID,
		UserID:             userID,
		Privacy:            input.Privacy,
		Notifications:      input.Notifications,
		DisplayAndContent:  input.DisplayAndContent,
		TelemetryVector:    nil, // Profil vierge
		TelemetryTags:      nil, // Profil vierge
		TelemetryTimestamp: 0,
		CreatedAt:          domain.TimeToMillis(now),
		UpdatedAt:          domain.TimeToMillis(now),
	}

	// 3. MISE EN CACHE IMMÉDIATE (Lecture instantanée L1 - USER & SPEED Caches)
	// --------------------------------------------------------

	// [USER CACHE]
	if err := cache_service.MarkUserTimelineEmpty(ctx, req.ID); err != nil {
		logger.Log.Warn().Err(err).Int64("user_id", req.ID).Msg("Echec initialisation Timeline ZSET")
	}

	// [SESSION CACHE]
	if err := cache_service.SetSessionInCache(ctx, sessions); err != nil {
		logger.Log.Warn().Err(err).Int64("session_id", sessions.ID).Msg("Echec USER Cache Redis Session")
	}

	// [SPEED CACHE]
	if err := cache_service.AddUserToSpeedCache(ctx, req, settings); err != nil {
		logger.Log.Warn().Err(err).Int64("user_id", req.ID).Msg("Echec SPEED Cache Redis User")
	}

	// [USER SETTINGS CACHE]
	if err := object_cache_service.SetUserSettings(ctx, settings); err != nil {
		logger.Log.Warn().Err(err).Int64("user_id", req.ID).Msg("Echec USER Cache Redis UserSettings")
	}

	// 4. PERSISTANCE ASYNCHRONE (Le "Write-Behind" vers L2/L3)
	// ---------------------------------------------

	if err := redis.EnqueueDB(ctx, userID, 0, redis.EntityUser, redis.ActionCreate, req, redis.TargetAll); err != nil {
		logger.Log.Error().Err(err).Int64("user_id", userID).Msg("CRITICAL: Impossible d'enqueue le User")
	}

	if err := redis.EnqueueDB(ctx, sessionID, userID, redis.EntitySession, redis.ActionCreate, sessions, redis.TargetAll); err != nil {
		logger.Log.Error().Err(err).Int64("session_id", sessionID).Msg("CRITICAL: Impossible d'enqueue la Session")
	}

	if err := redis.EnqueueDB(ctx, settingsID, userID, redis.EntityUserSettings, redis.ActionCreate, settings, redis.TargetAll); err != nil {
		logger.Log.Error().Err(err).Int64("settings_id", settingsID).Msg("CRITICAL: Impossible d'enqueue les UserSettings")
	}

	// 5. CUCKOO FILTERS (Prévention O(1) Mémoire)
	// --------------------------------------
	if cuckoo.GlobalCuckoo != nil {
		cuckoo.GlobalCuckoo.Insert([]byte("username:" + req.Username))
		cuckoo.GlobalCuckoo.Insert([]byte("email:" + req.Email))
		cuckoo.GlobalCuckoo.Insert([]byte("phone:" + req.Phone))
	}
	go func() {
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, "username", req.Username)
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, "email", req.Email)
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, "phone", req.Phone)
	}()

	// 6. RÉPONSE DÉFINITIVE PRÊTE À ÊTRE SÉRIALISÉE
	// --------------------------------------
	var avatarView media_models.MediaView
	if req.ProfilePictureID > 0 {
		if view, errMedia := media_service.GenerateMediaViewCascade(ctx, req.ProfilePictureID, userID, 0, userID); errMedia == nil {
			avatarView = view
		}
	}

	return auth_models.SignUpResponse{
		UserID:      userID,
		MasterToken: sessions.MasterToken,
		JWT:         newJWT,
		ExpiresAt:   sessions.ExpiresAt,
		Message:     "User created successfully",
		Avatar:      avatarView, // Injection saine par valeur
	}, nil
}
