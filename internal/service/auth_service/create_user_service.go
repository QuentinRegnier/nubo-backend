package auth_service

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// CreateUser orchestre l'inscription : règles métier, génération des modèles BDD,
// écriture asynchrone (Write-Behind) et upload de l'avatar.
func CreateUser(
	input auth_models.SignUpInput,
	ipAddress string,
	fileHeader *multipart.FileHeader,
	errFile error,
) (auth_models.SignUpResponse, error) {

	// 1. RÈGLES MÉTIER ET VÉRIFICATIONS D'UNICITÉ (BDD)
	// ---------------------------------------------------------
	if service.IsUnique(mongo.Users, "username", input.Username) == 0 {
		return auth_models.SignUpResponse{}, nubo_error.ErrUsernameTaken
	}
	if service.IsUnique(mongo.Users, "email", input.Email) == 0 {
		return auth_models.SignUpResponse{}, nubo_error.ErrEmailTaken
	}
	if service.IsUnique(mongo.Users, "phone", input.Phone) == 0 {
		return auth_models.SignUpResponse{}, nubo_error.ErrPhoneTaken
	}

	parsedBirthdate, err := time.Parse("02012006", input.Birthdate)
	if err != nil {
		return auth_models.SignUpResponse{}, nubo_error.ErrInvalidDate
	}

	age := time.Since(parsedBirthdate).Hours() / 24 / 365
	if age < 13 {
		return auth_models.SignUpResponse{}, nubo_error.ErrAgeUnder13
	}
	if age > 120 {
		return auth_models.SignUpResponse{}, nubo_error.ErrAgeOver120
	}

	var req auth_models.UserPayload
	if input.Gender != nil {
		g := *input.Gender
		if g < 0 || g > 2 {
			return auth_models.SignUpResponse{}, nubo_error.ErrInvalidGender
		}
		req.Sex = g
	}

	// 2. GÉNÉRATION DES DONNÉES (La "Vérité" absolue)
	// ---------------------------------------------------------
	now := time.Now().UTC()
	userID := pkg.GenerateID()
	sessionID := pkg.GenerateID()
	mediaID := pkg.GenerateID() // Média généré quoi qu'il arrive (peut être vide, mais l'UUID sécurise la ref)

	fmt.Printf("🆕 Création nouvel utilisateur avec ID Snowflake: %d\n", userID)
	fmt.Printf("🆕 Création nouvelle session avec ID Snowflake: %d\n", sessionID)

	// A. Hydratation du Payload Utilisateur
	req.ID = userID
	req.Username = input.Username
	req.Email = input.Email
	req.EmailVerified = false
	req.Phone = input.Phone
	req.PhoneVerified = false
	req.PasswordHash = input.PasswordHash
	req.FirstName = input.FirstName
	req.LastName = input.LastName
	req.Birthdate = parsedBirthdate
	req.Bio = pkg.CleanStr(input.Bio) // Nettoyage local du payload
	req.ProfilePictureID = mediaID
	req.Grade = 0
	req.Location = input.Location
	req.School = input.School
	req.Work = input.Work
	req.Badges = []string{}
	req.Desactivated = false
	req.Banned = false
	req.BanReason = ""
	req.BanExpiresAt = time.Time{}
	req.CreatedAt = now
	req.UpdatedAt = now

	// B. Hydratation du Payload Session
	var sessions models.SessionsRequest
	sessions.ID = sessionID
	sessions.UserID = userID
	sessions.DeviceToken = input.DeviceToken
	sessions.DeviceInfo = input.DeviceInfo
	sessions.IPHistory = []string{ipAddress}
	sessions.LastJWT = ""
	sessions.CreatedAt = now
	sessions.ExpiresAt = now.Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)
	sessions.ToleranceTime = now.Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second)

	// Génération des Tokens
	sessions.MasterToken, err = pkg.GenerateToken(req.ID, sessions.DeviceToken, variables.MasterTokenExpirationSeconds)
	if err != nil {
		return auth_models.SignUpResponse{}, fmt.Errorf("internal nubo_error (token generation): %w", err)
	}
	sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)
	sessions.LastSecret = sessions.DeviceToken

	newJWT, err := pkg.GenerateToken(req.ID, sessions.DeviceToken, variables.JWTExpirationSeconds)
	if err != nil {
		return auth_models.SignUpResponse{}, fmt.Errorf("internal nubo_error (jwt generation): %w", err)
	}

	// 3. MISE EN CACHE IMMÉDIATE (Lecture instantanée L1 - USER & SPEED Caches)
	// --------------------------------------------------------
	ctx := context.Background()

	// [USER CACHE] : On crée un ZSET vide pour la timeline de l'utilisateur (optimisation future)
	if err := cache_service.MarkUserTimelineEmpty(ctx, req.ID); err != nil {
		log.Printf("⚠️ Warning: Echec initialisation Timeline ZSET: %v", err)
	}

	// [SESSION CACHE] : Ajout de la session pour une validation rapide des futurs tokens
	if err := cache_service.SetSessionInCache(ctx, sessions); err != nil {
		log.Printf("⚠️ Warning: Echec USER Cache Redis Session: %v", err)
	}

	// [SPEED CACHE] : Indexation de l'utilisateur pour l'auto-complétion O(1)
	if err := cache_service.AddUserToSpeedCache(ctx, req); err != nil {
		log.Printf("⚠️ Warning: Echec SPEED Cache Redis User: %v", err)
	}

	// 4. PERSISTANCE ASYNCHRONE (Le "Write-Behind" vers L2/L3)
	// ---------------------------------------------

	// A. Enqueue Création User (PartitionKey à 0 pour hachage sur userID)
	if err := redis.EnqueueDB(ctx, userID, 0, redis.EntityUser, redis.ActionCreate, req, redis.TargetAll); err != nil {
		log.Printf("❌ CRITICAL: Impossible d'enqueue le User %d : %v", userID, err)
	}

	// B. Enqueue Création Session (PartitionKey forcée sur userID pour atterrir dans le même Shard)
	if err := redis.EnqueueDB(ctx, sessionID, userID, redis.EntitySession, redis.ActionCreate, sessions, redis.TargetAll); err != nil {
		log.Printf("❌ CRITICAL: Impossible d'enqueue la Session %d : %v", sessionID, err)
	}

	// C. Upload de l'avatar si présent
	if errFile == nil {
		file, err := fileHeader.Open()
		if err != nil {
			return auth_models.SignUpResponse{}, fmt.Errorf("cannot read file: %w", err)
		}

		go func() {
			defer func(file multipart.File) {
				err := file.Close()
				if err != nil {
					fmt.Println(err)
				}
			}(file)
			err = media_service.UploadMedia(file, userID, mediaID)
			if err != nil {
				log.Printf("internal nubo_error (image upload): %v", err)
			}
		}()
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
	return auth_models.SignUpResponse{
		UserID:           userID,
		MasterToken:      sessions.MasterToken,
		JWT:              newJWT,
		ExpiresAt:        sessions.ExpiresAt,
		Message:          "User created successfully",
		ProfilePictureID: req.ProfilePictureID, // On renvoie l'UUID généré au front
	}, nil
}
