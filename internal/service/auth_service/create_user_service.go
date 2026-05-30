package auth_service

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

func CreateUser(
	req *models.UserRequest,
	sessions *models.SessionsRequest,
	fileHeader *multipart.FileHeader,
	errFile error,
) (int64, string, error) { // Note: J'ai changé le retour en int64 car Snowflake génère des int64

	// 1. GÉNÉRATION DES DONNÉES (La "Vérité" est ici, dans Go)
	// ---------------------------------------------------------
	now := time.Now().UTC()

	// Génération des IDs Snowflake (Plus besoin de demander à Postgres)
	userID := pkg.GenerateID()
	fmt.Printf("🆕 Création nouvel utilisateur avec ID Snowflake: %d\n", userID)
	sessionID := pkg.GenerateID()
	fmt.Printf("🆕 Création nouvelle session avec ID Snowflake: %d\n", sessionID)

	// Préparation de l'objet User
	req.ID = userID
	req.CreatedAt = now
	req.UpdatedAt = now

	// Gestion de l'avatar (upload si présent)
	mediaID := pkg.GenerateID() // <--- ID Snowflake
	req.ProfilePictureID = mediaID

	// Préparation de l'objet Session
	sessions.ID = sessionID
	sessions.UserID = userID
	sessions.CreatedAt = now

	// Génération des Tokens (inchangé, mais utilise les nouveaux IDs)
	var err error
	sessions.MasterToken, err = pkg.GenerateToken(req.ID, sessions.DeviceToken, variables.MasterTokenExpirationSeconds)
	if err != nil {
		return -1, "", err
	}
	sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)

	// 2. MISE EN CACHE IMMÉDIATE (Lecture rapide pour le user)
	// --------------------------------------------------------
	// On écrit dans Redis tout de suite pour que le user puisse se loguer/voir son profil
	// sans attendre le worker Postgres.
	if err := redis.RedisCreateUser(*req); err != nil {
		log.Printf("⚠️ Warning: Echec cache_service Redis User: %v", err)
	}
	if err := redis.RedisCreateSession(*sessions); err != nil {
		log.Printf("⚠️ Warning: Echec cache_service Redis Session: %v", err)
	}

	// 3. PERSISTANCE ASYNCHRONE (Le "Write-Behind")
	// ---------------------------------------------
	ctx := context.Background() // Contexte pour Redis

	// A. Enqueue Création User (Mongo + Postgres)
	err = redis.EnqueueDB(ctx, userID, 0, redis.EntityUser, redis.ActionCreate, req, redis.TargetAll)
	if err != nil {
		log.Printf("❌ CRITICAL: Impossible d'enqueue le User %d : %v", userID, err)
		// Ici, tu pourrais décider de renvoyer une erreur, ou de retry
	}

	// B. Enqueue Création Session (Mongo + Postgres)
	// Note: Assure-toi d'avoir EntitySession défini dans tes constantes
	err = redis.EnqueueDB(ctx, sessionID, userID, redis.EntitySession, redis.ActionCreate, sessions, redis.TargetAll)
	if err != nil {
		log.Printf("❌ CRITICAL: Impossible d'enqueue la Session %d : %v", sessionID, err)
	}

	// C. Upload de l'avatar si présent
	if errFile == nil {
		file, err := fileHeader.Open()
		if err != nil {
			return -1, "", fmt.Errorf("cannot read file: %w", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Printf("Error closing file: %v", err)
			}
		}()

		// On récupère l'ID entier de la BDD
		go func() {
			err = service.UploadMedia(file, userID, mediaID)
			if err != nil {
				log.Printf("internal error (image upload): %v", err)
				return
			}
		}()
	}

	// 4. CUCKOO FILTERS (Mémoire - inchangé)
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

	newJWT, err := pkg.GenerateToken(req.ID, sessions.DeviceToken, variables.JWTExpirationSeconds)
	if err != nil {
		return 0, "", err
	}

	// 5. RETOUR INSTANTANÉ
	return userID, newJWT, nil
}
