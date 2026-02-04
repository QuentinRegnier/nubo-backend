package service

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	postgresgo "github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

func CreateUser(
	req *domain.UserRequest,
	sessions *domain.SessionsRequest,
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
		log.Printf("⚠️ Warning: Echec cache Redis User: %v", err)
	}
	if err := redis.RedisCreateSession(*sessions); err != nil {
		log.Printf("⚠️ Warning: Echec cache Redis Session: %v", err)
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
			err = UploadMedia(file, "profile_"+req.Username, userID, mediaID)
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

func Login(
	input domain.LoginInput,
	IPAddress []string,
) (domain.UserRequest, domain.SessionsRequest, string, error) {
	fmt.Printf("\n🚀 SERVICE LOGIN APPELÉ pour l'email : [%s]\n", input.Email)

	var user domain.UserRequest
	var sessions domain.SessionsRequest
	var err error

	// ---------------------------------------------------------
	// 1. CHARGEMENT DE L'UTILISATEUR (Fallback: Redis -> Mongo -> Postgres)
	// ---------------------------------------------------------

	// A. Essai Redis
	user, err = redis.RedisLoadUser(-1, "", input.Email, "")
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
			if errAdd := redis.RedisCreateUser(user); errAdd != nil {
				log.Printf("⚠️ Warning: Echec cache Redis User: %v", errAdd)
			}
		}
	}

	// C. Essai Postgres (si pas dans Mongo)
	if user.ID == 0 {
		fmt.Println("🔸 Mongo: User non trouvé, passage à Postgres...")
		user, err = postgresgo.FuncLoadUser(-1, "", input.Email, "")
		if err != nil {
			fmt.Printf("❌ ERREUR CRITIQUE APPEL POSTGRES : %v\n", err)
			return domain.UserRequest{}, domain.SessionsRequest{}, "", err
		}

		if user.ID == 0 {
			fmt.Printf("❌ ERREUR : Utilisateur introuvable dans AUCUNE base pour l'email '%s'\n", input.Email)
			return domain.UserRequest{}, domain.SessionsRequest{}, "", domain.ErrNotFound
		}

		fmt.Println("✅ Utilisateur trouvé dans Postgres !")
		fmt.Println("User loaded from Postgres:", user)
		if errAdd := redis.RedisCreateUser(user); errAdd != nil {
			log.Printf("⚠️ Warning: Echec cache Redis User: %v", errAdd)
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
		return domain.UserRequest{}, domain.SessionsRequest{}, "", domain.ErrInvalidCredentials
	}

	// Vérification Statut Compte
	if user.Desactivated || user.Banned {
		if user.Desactivated {
			return domain.UserRequest{}, domain.SessionsRequest{}, "", domain.ErrDesactivated
		}
		return domain.UserRequest{}, domain.SessionsRequest{}, "", domain.ErrBanned
	}

	fmt.Println("🔓 MOT DE PASSE VALIDE ! Gestion de la session...")

	// ---------------------------------------------------------
	// 3. GESTION DE LA SESSION (Load or Create)
	// ---------------------------------------------------------

	ctx := context.Background()
	now := time.Now().UTC()
	isNewSession := false
	DeviceToken := input.DeviceToken

	// A. TENTATIVE DE RECUPERATION (Redis -> Mongo -> Postgres)
	// On essaie de trouver une session active pour ce device
	sessions, _ = redis.RedisLoadSession(user.ID, DeviceToken, "", "")
	if sessions.ID == 0 {
		sessions, _ = mongo.MongoLoadSession(user.ID, DeviceToken, "", "")
		if sessions.ID == 0 {
			sessions, _ = postgresgo.FuncLoadSession(-1, user.ID, DeviceToken, "")
			if sessions.ID == 0 {
				fmt.Println("🔸 Postgres: Session non trouvée, création d'une nouvelle session...")
			} else {
				if err := redis.RedisCreateSession(sessions); err != nil {
					log.Printf("⚠️ Warning: Echec mise à jour cache Redis Session: %v", err)
				}
				if err := redis.EnqueueDB(ctx, sessions.ID, 0, redis.EntitySession, redis.ActionCreate, sessions, redis.TargetMongo); err != nil {
					log.Printf("⚠️ Warning: Echec mise à jour cache Redis Session: %v", err)
				}
				fmt.Println("✅ Session trouvée dans Postgres")
			}
		} else {
			if err := redis.RedisCreateSession(sessions); err != nil {
				log.Printf("⚠️ Warning: Echec mise à jour cache Redis Session: %v", err)
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
			return domain.UserRequest{}, domain.SessionsRequest{}, "", domain.ErrInvalidIPAddress
		}
	}

	// C. ROTATION DES TOKENS (Ratchet Algorithm) - Commun aux deux cas
	sessions.ExpiresAt = now.Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)

	// Génération nouveau Master Token
	sessions.MasterToken, err = pkg.GenerateToken(user.ID, DeviceToken, variables.MasterTokenExpirationSeconds)
	if err != nil {
		return domain.UserRequest{}, domain.SessionsRequest{}, "", err
	}

	// Dérivation des secrets (Sécurité)
	sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)
	sessions.LastSecret = sessions.DeviceToken // Reset du cycle
	sessions.LastJWT = ""
	sessions.ToleranceTime = time.Time{}

	// ---------------------------------------------------------
	// 4. SAUVEGARDE & PERSISTANCE (Architecture Write-Behind)
	// ---------------------------------------------------------

	// A. CACHE REDIS (Immédiat)
	// On utilise toujours RedisCreateSession ici (qui fait un SET) pour écraser le cache avec les nouvelles infos fraîches
	if isNewSession {
		if err := redis.RedisCreateSession(sessions); err != nil {
			log.Printf("⚠️ Warning: Echec mise à jour cache Redis Session: %v", err)
		}
	} else {
		if err := redis.RedisUpdateSession(sessions); err != nil {
			log.Printf("⚠️ Warning: Echec mise à jour cache Redis Session: %v", err)
		}
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
		// L'utilisateur pourra quand même se connecter grâce au cache Redis mis à jour juste au-dessus.
		log.Printf("❌ CRITICAL: Impossible d'enqueue la Session %d : %v", sessions.ID, err)
	} else {
		log.Printf("✅ Session %d mise en file d'attente (Action: %s)", sessions.ID, action)
	}

	newJWT, err := pkg.GenerateToken(user.ID, sessions.DeviceToken, variables.JWTExpirationSeconds)
	if err != nil {
		return domain.UserRequest{}, domain.SessionsRequest{}, "", err
	}

	return user, sessions, newJWT, nil
}

// startWebsocket
// func StartWebsocket() {}
