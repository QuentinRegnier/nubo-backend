package auth_service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	postgresgo "github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// Login retourne uniquement les tokens et l'ID utilisateur. Les métadonnées sont déléguées à /sync.
func Login(
	input auth_models.LoginInput,
	IPAddress []string,
) (int64, auth_models.SessionsPayload, string, error) {
	logger.Log.Info().Str("email", input.Email).Msg("Tentative de connexion")

	var user auth_models.UserPayload
	var sessions auth_models.SessionsPayload
	var err error
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// 1. CHARGEMENT DE L'UTILISATEUR (L2 -> L3)
	// -------------------------------------------------------------------------
	user, err = mongo.MongoLoadUser(-1, "", input.Email, "")
	if err != nil {
		logger.Log.Warn().Err(err).Str("email", input.Email).Msg("Mongo: utilisateur absent ou erreur")
	}

	if user.ID == 0 {
		user, err = postgresgo.FuncLoadUser(-1, "", input.Email, "")
		if err != nil {
			return -1, auth_models.SessionsPayload{}, "", nubo_error.NewInternal(err)
		}

		if user.ID == 0 {
			return -1, auth_models.SessionsPayload{}, "", nubo_error.NewNotFound("USER_NOT_FOUND", "Identifiants incorrects.", nil) // On ne dit pas "email non trouvé" pour des raisons de sécu
		}

		if errQueue := redis.EnqueueDB(ctx, user.ID, 0, redis.EntityUser, redis.ActionCreate, &user, redis.TargetMongo); errQueue != nil {
			logger.Log.Warn().Err(errQueue).Int64("user_id", user.ID).Msg("Échec de la mise en file d'attente MongoDB pour l'utilisateur")
		}
	}

	// 2. CONTRÔLE SÉCURITÉ ET STATUT DU COMPTE
	if strings.TrimSpace(user.PasswordHash) != strings.TrimSpace(input.PasswordHash) {
		return -1, auth_models.SessionsPayload{}, "", nubo_error.NewForbidden("INVALID_CREDENTIALS", "Identifiants incorrects.", nil)
	}

	if user.Desactivated || user.Banned {
		if user.Desactivated {
			return -1, auth_models.SessionsPayload{}, "", nubo_error.NewForbidden("ACCOUNT_DEACTIVATED", "Ce compte est désactivé.", nil)
		}
		return -1, auth_models.SessionsPayload{}, "", nubo_error.NewForbidden("ACCOUNT_BANNED", "Ce compte est banni.", nil)
	}

	// -------------------------------------------------------------------------
	// 3. GESTION DE LA SESSION DE L'APPAREIL (Hot Data)
	// -------------------------------------------------------------------------
	now := time.Now().UTC()
	isNewSession := false
	firebaseInstallationID := input.FirebaseInstallationID

	sessions, _ = cache_service.LoadSessionFromCache(ctx, user.ID, firebaseInstallationID, "")
	if sessions.ID == 0 {
		sessions, _ = mongo.MongoLoadSession(user.ID, firebaseInstallationID, "", "")
		if sessions.ID == 0 {
			sessions, _ = postgresgo.FuncLoadSession(-1, user.ID, firebaseInstallationID, "")

			if sessions.ID != 0 {
				// ⬆️ PROMOTION L3 -> L2 (Asynchrone via Worker)
				go func(s auth_models.SessionsPayload) {
					bgCtx := context.Background()
					_ = redis.EnqueueDB(bgCtx, s.ID, s.UserID, redis.EntitySession, redis.ActionUpdate, s, redis.TargetMongo)
				}(sessions)
			}
		}

		if sessions.ID != 0 {
			// ⬆️ PROMOTION L3/L2 -> L1 (Immédiate en RAM)
			_ = cache_service.SetSessionInCache(ctx, sessions)
		}
	}

	if sessions.ID != 0 {
		sessions.DeviceInfo = input.DeviceInfo
		if len(IPAddress) > 0 && !pkg.Exists(sessions.IPHistory, IPAddress[0]) {
			sessions.IPHistory = append(sessions.IPHistory, IPAddress[0])
		}
	} else {
		isNewSession = true
		sessions.ID = pkg.GenerateID()
		sessions.UserID = user.ID
		sessions.CreatedAt = now
		sessions.FirebaseInstallationID = firebaseInstallationID
		sessions.DeviceInfo = input.DeviceInfo

		if len(IPAddress) > 0 {
			sessions.IPHistory = []string{IPAddress[0]}
		} else {
			return -1, auth_models.SessionsPayload{}, "", nubo_error.NewBadRequest("INVALID_IP", "Adresse IP requise pour la connexion.", nil)
		}
	}

	sessions.ExpiresAt = now.Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)
	sessions.MasterToken, err = pkg.GenerateToken(user.ID, firebaseInstallationID, variables.MasterTokenExpirationSeconds)
	if err != nil {
		return -1, auth_models.SessionsPayload{}, "", nubo_error.NewInternal(err)
	}

	sessions.CurrentSecret = security.DeriveNextSecret(sessions.FirebaseInstallationID, sessions.MasterToken, sessions.MasterToken, sessions.FirebaseInstallationID)
	sessions.LastSecret = sessions.FirebaseInstallationID
	sessions.LastJWT = ""
	sessions.ToleranceTime = time.Time{}

	newJWT, err := pkg.GenerateToken(user.ID, sessions.FirebaseInstallationID, variables.JWTExpirationSeconds)
	if err != nil {
		return -1, auth_models.SessionsPayload{}, "", nubo_error.NewInternal(err)
	}

	// 4. SYNCHRONISATION DES COUCHES DE CACHE L1 & ALIGNEMENT DE VITESSE
	if errSet := cache_service.SetSessionInCache(ctx, sessions); errSet != nil {
		logger.Log.Warn().Err(errSet).Int64("session_id", sessions.ID).Msg("Échec mise en cache L1 de la Session")
	}

	timelineKey := fmt.Sprintf("profile:posts:zset:%d", user.ID)
	timelineExists, errTimeline := redis.Exists(ctx, timelineKey)
	if errTimeline != nil || !timelineExists {
		_ = cache_service.MarkUserTimelineEmpty(ctx, user.ID)
	}

	settings, _ := object_cache_service.GetUserSettingsCascade(ctx, user.ID)

	var liteUser lite_models.UserLiteRequest
	if errSpeed := redis.UsersLite.GetObject(ctx, user.ID, &liteUser); errSpeed != nil {
		uReq := auth_models.UserPayload{
			ID:               user.ID,
			Username:         user.Username,
			FirstName:        user.FirstName,
			LastName:         user.LastName,
			ProfilePictureID: user.ProfilePictureID,
		}
		_ = cache_service.AddUserToSpeedCache(ctx, uReq, settings)
	}

	// 5. ENREGISTREMENT SUR LA QUEUE DE PERSISTANCE (Write-Behind)
	action := redis.ActionUpdate
	if isNewSession {
		action = redis.ActionCreate
	}
	err = redis.EnqueueDB(ctx, sessions.ID, user.ID, redis.EntitySession, action, sessions, redis.TargetAll)
	if err != nil {
		logger.Log.Error().Err(err).Int64("session_id", sessions.ID).Msg("Rupture du Write-Behind pour la session")
	}

	return user.ID, sessions, newJWT, nil
}
