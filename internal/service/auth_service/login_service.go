package auth_service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	postgresgo "github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

func Login(
	input auth_models.LoginInput,
	IPAddress []string,
) (models.UserRequest, models.SessionsRequest, string, error) {
	fmt.Printf("\n🚀 SERVICE LOGIN APPELÉ pour l'email : [%s]\n", input.Email)

	var user models.UserRequest
	var sessions models.SessionsRequest
	var err error

	// ---------------------------------------------------------
	// 1. CHARGEMENT DE L'UTILISATEUR (Fallback: Redis -> Mongo -> Postgres)
	// ---------------------------------------------------------

	// A. Essai Cache
	user, err = cache_service.LoadUserFullFromCache(context.Background(), -1, "", input.Email, "")
	if err != nil {
		fmt.Printf("🔸 Redis: erreur (%v)\n", err)
	} else {
		fmt.Println("✅ Utilisateur trouvé dans Redis !")
		fmt.Println("User loaded from Redis:", user)
	}

	// B. Essai Mongo (si pas dans Redis)
	if user.ID == 0 {
		fmt.Println("🔸 Redis: User non trouvé, passage à Mongo...")
		user, err = mongo.MongoLoadUser(-1, "", input.Email, "")
		if err != nil {
			fmt.Printf("🔸 Mongo: erreur (%v)\n", err)
		} else {
			fmt.Println("✅ Utilisateur trouvé dans Mongo !")
			fmt.Println("User loaded from Mongo:", user)
			if errAdd := cache_service.SetUserFullInCache(context.Background(), user); errAdd != nil {
				log.Printf("⚠️ Warning: Echec cache_service Redis User: %v", errAdd)
			}
		}
	}

	// C. Essai Postgres (si pas dans Mongo)
	if user.ID == 0 {
		fmt.Println("🔸 Mongo: User non trouvé, passage à Postgres...")
		user, err = postgresgo.FuncLoadUser(-1, "", input.Email, "")
		if err != nil {
			fmt.Printf("❌ ERREUR CRITIQUE APPEL POSTGRES : %v\n", err)
			return models.UserRequest{}, models.SessionsRequest{}, "", err
		}

		if user.ID == 0 {
			fmt.Printf("❌ ERREUR : Utilisateur introuvable dans AUCUNE base pour l'email '%s'\n", input.Email)
			return models.UserRequest{}, models.SessionsRequest{}, "", domain.ErrNotFound
		}

		fmt.Println("✅ Utilisateur trouvé dans Postgres !")
		fmt.Println("User loaded from Postgres:", user)
		if errAdd := cache_service.SetUserFullInCache(context.Background(), user); errAdd != nil {
			log.Printf("⚠️ Warning: Echec cache_service Redis User: %v", errAdd)
		}
		if err := redis.EnqueueDB(context.Background(), user.ID, 0, redis.EntityUser, redis.ActionCreate, &user, redis.TargetMongo); err != nil {
			log.Printf("Error enqueuing to DB: %v", err)
		}
	}

	// ---------------------------------------------------------
	// 2. VÉRIFICATION DU MOT DE PASSE
	// ---------------------------------------------------------

	fmt.Println("✅ UTILISATEUR TROUVÉ ! Analyse du mot de passe...")
	// TRIM pour sécurité (espaces fantômes en BDD)
	if strings.TrimSpace(user.PasswordHash) != strings.TrimSpace(input.PasswordHash) {
		fmt.Println("🔒 MOT DE PASSE INCORRECT")
		return models.UserRequest{}, models.SessionsRequest{}, "", domain.ErrInvalidCredentials
	}

	// Vérification Statut Compte
	if user.Desactivated || user.Banned {
		if user.Desactivated {
			return models.UserRequest{}, models.SessionsRequest{}, "", domain.ErrDesactivated
		}
		return models.UserRequest{}, models.SessionsRequest{}, "", domain.ErrBanned
	}

	fmt.Println("🔓 MOT DE PASSE VALIDE ! Gestion de la session...")

	// ---------------------------------------------------------
	// 3. GESTION DE LA SESSION (Load or Create)
	// ---------------------------------------------------------

	ctx := context.Background()
	now := time.Now().UTC()
	isNewSession := false
	DeviceToken := input.DeviceToken

	// A. TENTATIVE DE RECUPERATION (Cache -> Mongo -> Postgres)
	// On essaie de trouver une session active pour ce device
	sessions, _ = cache_service.LoadSessionFromCache(context.Background(), user.ID, DeviceToken, "", "")
	if sessions.ID == 0 {
		sessions, _ = mongo.MongoLoadSession(user.ID, DeviceToken, "", "")
		if sessions.ID == 0 {
			sessions, _ = postgresgo.FuncLoadSession(-1, user.ID, DeviceToken, "")
			if sessions.ID == 0 {
				fmt.Println("🔸 Postgres: Session non trouvée, création d'une nouvelle session...")
			} else {
				if err := cache_service.SetSessionInCache(context.Background(), sessions); err != nil {
					log.Printf("⚠️ Warning: Echec mise à jour cache_service Redis Session: %v", err)
				}
				if err := redis.EnqueueDB(ctx, sessions.ID, 0, redis.EntitySession, redis.ActionCreate, sessions, redis.TargetMongo); err != nil {
					log.Printf("⚠️ Warning: Echec mise à jour cache_service Redis Session: %v", err)
				}
				fmt.Println("✅ Session trouvée dans Postgres")
			}
		} else {
			if err := cache_service.SetSessionInCache(context.Background(), sessions); err != nil {
				log.Printf("⚠️ Warning: Echec mise à jour cache_service Redis Session: %v", err)
			}
			fmt.Println("✅ Session trouvée dans Mongo")
		}
	} else {
		fmt.Println("✅ Session trouvée dans Redis")
	}

	// B. MISE A JOUR OU CRÉATION
	if sessions.ID != 0 {
		// --- UPDATE EXISTING ---
		// On met à jour l'historique IP
		sessions.DeviceInfo = input.DeviceInfo
		if len(IPAddress) > 0 && !pkg.Exists(sessions.IPHistory, IPAddress[0]) {
			sessions.IPHistory = append(sessions.IPHistory, IPAddress[0])
		}
	} else {
		// --- CREATE NEW ---
		isNewSession = true

		// Génération ID Snowflake & Dates (C'est Go qui décide !)
		sessions.ID = pkg.GenerateID()
		sessions.UserID = user.ID
		sessions.CreatedAt = now

		sessions.DeviceToken = DeviceToken
		sessions.DeviceInfo = input.DeviceInfo

		if len(IPAddress) > 0 {
			sessions.IPHistory = []string{IPAddress[0]}
		} else {
			return models.UserRequest{}, models.SessionsRequest{}, "", domain.ErrInvalidIPAddress
		}
	}

	// C. ROTATION DES TOKENS (Ratchet Algorithm) - Commun aux deux cas
	sessions.ExpiresAt = now.Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)

	// Génération nouveau Master Token
	sessions.MasterToken, err = pkg.GenerateToken(user.ID, DeviceToken, variables.MasterTokenExpirationSeconds)
	if err != nil {
		return models.UserRequest{}, models.SessionsRequest{}, "", err
	}

	// Dérivation des secrets (Sécurité)
	sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)
	sessions.LastSecret = sessions.DeviceToken // Reset du cycle
	sessions.LastJWT = ""
	sessions.ToleranceTime = time.Time{}

	// ---------------------------------------------------------
	// 4. SAUVEGARDE & PERSISTANCE (Architecture Write-Behind)
	// ---------------------------------------------------------

	// A. CACHE (Immédiat)
	// L'opération SET écrase l'ancienne valeur. On utilise donc SetSessionInCache dans les deux cas.
	if err := cache_service.SetSessionInCache(context.Background(), sessions); err != nil {
		log.Printf("⚠️ Warning: Echec mise à jour cache_service Session: %v", err)
	}
	// B. PERSISTANCE ASYNCHRONE (Vers Mongo & Postgres)
	// On choisit l'action : CREATE (si nouveau) ou UPDATE (si existant)
	action := redis.ActionUpdate
	if isNewSession {
		action = redis.ActionCreate
	}

	// On envoie dans la file d'attente. Le worker s'occupera d'écrire dans PG et Mongo.
	// TargetAll = On veut écrire dans les deux bases.
	err = redis.EnqueueDB(
		ctx,
		sessions.ID,
		0,
		redis.EntitySession,
		action,
		sessions,
		redis.TargetAll,
	)

	if err != nil {
		// C'est rare (Redis down), mais il faut le logger en critique.
		// L'utilisateur pourra quand même se connecter grâce au cache_service Redis mis à jour juste au-dessus.
		log.Printf("❌ CRITICAL: Impossible d'enqueue la Session %d : %v", sessions.ID, err)
	} else {
		log.Printf("✅ Session %d mise en file d'attente (Action: %s)", sessions.ID, action)
	}

	newJWT, err := pkg.GenerateToken(user.ID, sessions.DeviceToken, variables.JWTExpirationSeconds)
	if err != nil {
		return models.UserRequest{}, models.SessionsRequest{}, "", err
	}

	return user, sessions, newJWT, nil
}
