package service

import (
	"log"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	postgresgo "github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// addImage gère l'upload (S3/Disque)
func AddImage(rawBinaryData string, name string) int {

	// 1. Transformation en Reader (Zero-Allocation)
	// strings.NewReader est beaucoup plus rapide que bytes.NewReader([]byte(s))
	// car il ne copie pas les données en mémoire.
	reader := strings.NewReader(rawBinaryData)

	// 2. Appel du service
	mediaID, err := UploadMedia(reader, name, "system")

	if err != nil {
		log.Printf("Erreur upload image '%s' : %v", name, err)
		return 0
	}

	return mediaID
}

func CreateUser(
	req domain.UserRequest,
	sessions domain.SessionsRequest,
) (int, error) {
	// Enregistrement dans PostgreSQL
	userID, createdAtUser, updatedAtUser, sessionID, createdAtSession, err := postgresgo.FuncCreateUser(req, sessions)
	if err != nil {
		return 0, err
	}
	if pkg.EstNonVide(req.ProfilePictureID) {

		// A. PostgreSQL : Appel direct de la procédure stockée
		_, err := postgres.PostgresDB.Exec("CALL content.proc_update_media_owner($1, $2)", req.ProfilePictureID, userID)
		if err != nil {
			log.Printf("⚠️ Erreur liaison avatar Postgres (MediaID: %v) : %v", req.ProfilePictureID, err)
		}

		// B. MongoDB : Utilisation de ta structure normalisée db.Media
		// On prépare le filtre (quel média modifier ?)
		filter := map[string]any{
			"id": req.ProfilePictureID,
		}
		// On prépare la mise à jour (quel champ changer ?)
		update := map[string]any{
			"owner_id": userID,
		}

		// On utilise ta méthode Update générique définie dans mongo_manage.go
		if err := mongo.Media.Update(filter, update); err != nil {
			log.Printf("⚠️ Erreur liaison avatar Mongo (MediaID: %v) : %v", req.ProfilePictureID, err)
		} else {
			log.Printf("✅ Avatar lié avec succès (PG + Mongo) à l'utilisateur %v", userID)
		}
	}

	// Enregistrement dans MongoDB
	req.ID = userID
	req.CreatedAt = createdAtUser
	req.UpdatedAt = updatedAtUser

	sessions.ID = sessionID
	sessions.UserID = userID
	sessions.CreatedAt = createdAtSession

	if err := mongo.MongoCreateUser(req); err != nil {
		log.Printf("Erreur Mongo CreateUser: %v", err)
	}
	if err := mongo.MongoCreateSession(sessions); err != nil {
		log.Printf("Erreur Mongo CreateSession: %v", err)
	}
	// Enregistrement dans Redis
	if err := redis.RedisCreateUser(req); err != nil {
		log.Printf("Warning: Echec cache Redis User: %v", err)
	}
	if err := redis.RedisCreateSession(sessions); err != nil {
		log.Printf("Warning: Echec cache Redis Session: %v", err)
	}

	// ---------------------------------------------------------
	// MISE À JOUR CUCKOO FILTER (Distribué via Redis Flux)
	// ---------------------------------------------------------
	// On ajoute les clés dans le filtre local ET on notifie les autres instances

	// 1. Mise à jour Locale (Immédiat)
	if cuckoo.GlobalCuckoo != nil {
		cuckoo.GlobalCuckoo.Insert([]byte("username:" + req.Username))
		cuckoo.GlobalCuckoo.Insert([]byte("email:" + req.Email))
		cuckoo.GlobalCuckoo.Insert([]byte("phone:" + req.Phone))
	}

	// 2. Broadcast Flux Redis (Pour les autres serveurs)
	go func() {
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, "username", req.Username)
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, "email", req.Email)
		cuckoo.BroadcastCuckooUpdate(cuckoo.ActionAdd, "phone", req.Phone)
	}()

	return userID, nil
}

func Login(
	input domain.LoginInput,
) (domain.UserRequest, domain.SessionsRequest, error) {
	var user domain.UserRequest
	var sessions domain.SessionsRequest
	var err error

	// Enregistrement dans Redis
	user, err = redis.RedisLoadUser(-1, "", input.Email, "")
	if err != nil {
		log.Printf("Erreur Redis LoadUser: %v", err)
	}
	if pkg.EstNonVide(user) {
		if user.PasswordHash == input.PasswordHash {
			sessions, err = redis.RedisLoadSession(user.ID, input.DeviceToken)
			if err != nil {
				log.Printf("Erreur Redis LoadSession: %v", err)
			}
			if pkg.EstNonVide(sessions) {
				return user, sessions, nil
			} else {
				// create session
				return user, sessions, nil
			}
		} else {
			return domain.UserRequest{}, domain.SessionsRequest{}, domain.ErrInvalidCredentials
		}
	} else {
		// Enregistrement dans MongoDB
		user, err = mongo.MongoLoadUser(-1, "", input.Email, "")
		if err != nil {
			log.Printf("Erreur Mongo CreateUser: %v", err)
		}
		if pkg.EstNonVide(user) {
			if user.PasswordHash == input.PasswordHash {
				sessions, err = mongo.MongoLoadSession(user.ID, input.DeviceToken)
				if err != nil {
					log.Printf("Erreur Mongo CreateSession: %v", err)
				}
				if pkg.EstNonVide(sessions) {
					return user, sessions, nil
				} else {
					// create session
					return user, sessions, nil
				}
			} else {
				return domain.UserRequest{}, domain.SessionsRequest{}, domain.ErrInvalidCredentials
			}
		} else {
			// Recherche dans PostgreSQL
			user, err = postgresgo.FuncLoadUser(-1, "", input.Email, "")
			if err != nil {
				return domain.UserRequest{}, domain.SessionsRequest{}, err
			}
			if pkg.EstNonVide(user) {
				if user.PasswordHash == input.PasswordHash {
					sessions, err = mongo.MongoLoadSession(user.ID, input.DeviceToken)
					if err != nil {
						log.Printf("Erreur Mongo CreateSession: %v", err)
					}
					if pkg.EstNonVide(sessions) {
						return user, sessions, nil
					} else {
						// create session
						return user, sessions, nil
					}
				} else {
					return domain.UserRequest{}, domain.SessionsRequest{}, domain.ErrInvalidCredentials
				}
			} else {
				return domain.UserRequest{}, domain.SessionsRequest{}, domain.ErrNotFound
			}
		}
	}
}

// startWebsocket
func StartWebsocket() {
	// Logique WS ici
}
