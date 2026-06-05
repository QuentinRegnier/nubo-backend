package auth_service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	postgresgo "github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// Login prend désormais en charge le tuple de retour incluant l'URL de l'avatar
func Login(
	input auth_models.LoginInput,
	IPAddress []string,
) (auth_models.UserPayload, models.SessionsRequest, string, string, error) {
	fmt.Printf("\n🚀 SERVICE LOGIN APPELÉ pour l'identifiant : [%s]\n", input.Email)

	var user auth_models.UserPayload
	var sessions models.SessionsRequest
	var err error
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// 1. CHARGEMENT DE L'UTILISATEUR (Bases de Persistance uniquement)
	// -------------------------------------------------------------------------

	// A. Requête vers MongoDB (Niveau L2)
	user, err = mongo.MongoLoadUser(-1, "", input.Email, "")
	if err != nil {
		fmt.Printf("🔸 Mongo: utilisateur absent ou erreur (%v)\n", err)
	}

	// B. Fallback vers PostgreSQL (Niveau L3)
	if user.ID == 0 {
		fmt.Println("🔸 Passage de contrôle à PostgreSQL...")
		user, err = postgresgo.FuncLoadUser(-1, "", input.Email, "")
		if err != nil {
			return auth_models.UserPayload{}, models.SessionsRequest{}, "", "", fmt.Errorf("postgres critical failure: %w", err)
		}

		if user.ID == 0 {
			return auth_models.UserPayload{}, models.SessionsRequest{}, "", "", nubo_error.ErrNotFound
		}

		// Alignement de synchronisation asynchrone pour consolider le stockage Mongo
		if errQueue := redis.EnqueueDB(ctx, user.ID, 0, redis.EntityUser, redis.ActionCreate, &user, redis.TargetMongo); errQueue != nil {
			log.Printf("⚠️ Warning: Échec de la mise en file d'attente MongoDB pour l'utilisateur: %v", errQueue)
		}
	}

	// -------------------------------------------------------------------------
	// 2. CONTRÔLE SÉCURITÉ ET STATUT DU COMPTE
	// -------------------------------------------------------------------------
	if strings.TrimSpace(user.PasswordHash) != strings.TrimSpace(input.PasswordHash) {
		return auth_models.UserPayload{}, models.SessionsRequest{}, "", "", nubo_error.ErrInvalidCredentials
	}

	if user.Desactivated || user.Banned {
		if user.Desactivated {
			return auth_models.UserPayload{}, models.SessionsRequest{}, "", "", nubo_error.ErrDesactivated
		}
		return auth_models.UserPayload{}, models.SessionsRequest{}, "", "", nubo_error.ErrBanned
	}

	// -------------------------------------------------------------------------
	// 3. GESTION DE LA SESSION DE L'APPAREIL (Hot Data)
	// -------------------------------------------------------------------------
	now := time.Now().UTC()
	isNewSession := false
	deviceToken := input.DeviceToken

	// Recherche de session active (User Cache L1 -> Mongo L2 -> Postgres L3)
	sessions, _ = cache_service.LoadSessionFromCache(ctx, user.ID, deviceToken, "")
	if sessions.ID == 0 {
		sessions, _ = mongo.MongoLoadSession(user.ID, deviceToken, "", "")
		if sessions.ID == 0 {
			sessions, _ = postgresgo.FuncLoadSession(-1, user.ID, deviceToken, "")
		}
	}

	// Traitement structurel de la session
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
		sessions.DeviceToken = deviceToken
		sessions.DeviceInfo = input.DeviceInfo

		if len(IPAddress) > 0 {
			sessions.IPHistory = []string{IPAddress[0]}
		} else {
			return auth_models.UserPayload{}, models.SessionsRequest{}, "", "", nubo_error.ErrInvalidIPAddress
		}
	}

	// Algorithme de rotation Ratchet (Cryptographie)
	sessions.ExpiresAt = now.Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)
	sessions.MasterToken, err = pkg.GenerateToken(user.ID, deviceToken, variables.MasterTokenExpirationSeconds)
	if err != nil {
		return auth_models.UserPayload{}, models.SessionsRequest{}, "", "", fmt.Errorf("token generation error: %w", err)
	}

	sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)
	sessions.LastSecret = sessions.DeviceToken
	sessions.LastJWT = ""
	sessions.ToleranceTime = time.Time{}

	newJWT, err := pkg.GenerateToken(user.ID, sessions.DeviceToken, variables.JWTExpirationSeconds)
	if err != nil {
		return auth_models.UserPayload{}, models.SessionsRequest{}, "", "", fmt.Errorf("jwt generation error: %w", err)
	}

	// -------------------------------------------------------------------------
	// 4. SYNCHRONISATION DES COUCHES DE CACHE L1 & ALIGNEMENT DE VITESSE
	// -------------------------------------------------------------------------

	// A. [SESSION CACHE] : Sauvegarde immédiate en RAM
	if errSet := cache_service.SetSessionInCache(ctx, sessions); errSet != nil {
		log.Printf("⚠️ Warning: Échec mise en cache de la Session %d : %v", sessions.ID, errSet)
	}

	// B. [USER CACHE TIMELINE] : Validation systématique de l'existence de la grille de posts (ZSET)
	// Utilisation de la clé d'index "profile" pour correspondre au moteur de lecture
	timelineKey := fmt.Sprintf("profile:posts:zset:%d", user.ID)
	timelineExists, errTimeline := redis.Exists(ctx, timelineKey)
	if errTimeline != nil || !timelineExists {
		// Initialisation du bouchon anti-martèlement (-1) en cas d'absence
		_ = cache_service.MarkUserTimelineEmpty(ctx, user.ID)
	}

	// C. [SPEED CACHE] : Garantie de la présence des métadonnées compressées publiques (UserLite)
	var liteUser models.UserLiteRequest
	if errSpeed := redis.UsersLite.GetObject(ctx, user.ID, &liteUser); errSpeed != nil {
		// Reconstruction instantanée de la structure Lite en cas de nettoyage ou d'expulsion de la RAM
		uReq := auth_models.UserPayload{
			ID:               user.ID,
			Username:         user.Username,
			FirstName:        user.FirstName,
			LastName:         user.LastName,
			ProfilePictureID: user.ProfilePictureID,
		}
		_ = cache_service.AddUserToSpeedCache(ctx, uReq)
	}

	// 🔐 D. [PROFILE PICTURE URL] : Extraction du storage path et signature du lien
	var profilePictureURL string
	if user.ProfilePictureID != 0 {
		// On interroge la cascade L1->L2->L3 pour obtenir le storage path de l'avatar
		mediaPayload, errMedia := media_service.GetMediaCascade(ctx, user.ProfilePictureID)
		if errMedia == nil && mediaPayload.Visibility {
			// On génère l'URL signée avec le lecteur, l'auteur (lui-même) et l'ID de post à 0
			profilePictureURL = media_service.GenerateWatermarkedURL(
				mediaPayload.StoragePath,
				user.ID,
				0,
				user.ID,
			)
		}
	}

	// -------------------------------------------------------------------------
	// 5. ENREGISTREMENT SUR LA QUEUE DE PERSISTANCE (Write-Behind)
	// -------------------------------------------------------------------------
	action := redis.ActionUpdate
	if isNewSession {
		action = redis.ActionCreate
	}

	err = redis.EnqueueDB(ctx, sessions.ID, user.ID, redis.EntitySession, action, sessions, redis.TargetAll)
	if err != nil {
		log.Printf("❌ CRITICAL: Rupture du Write-Behind pour la session %d : %v", sessions.ID, err)
	}

	return user, sessions, newJWT, profilePictureURL, nil
}
