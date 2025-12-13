package data

import (
	"log"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/cache"
	"github.com/QuentinRegnier/nubo-backend/internal/db"
	"github.com/QuentinRegnier/nubo-backend/internal/media"
	"github.com/QuentinRegnier/nubo-backend/internal/tools"
)

// addImage gère l'upload (S3/Disque)
func AddImage(rawBinaryData string, name string) int {

	// 1. Transformation en Reader (Zero-Allocation)
	// strings.NewReader est beaucoup plus rapide que bytes.NewReader([]byte(s))
	// car il ne copie pas les données en mémoire.
	reader := strings.NewReader(rawBinaryData)

	// 2. Appel du service
	mediaID, err := media.UploadMedia(reader, name, "system")

	if err != nil {
		log.Printf("Erreur upload image '%s' : %v", name, err)
		return 0
	}

	return mediaID
}

func CreateUser(
	req tools.UserRequest,
	sessions tools.SessionsRequest,
) (int, error) {
	// Enregistrement dans PostgreSQL
	userID, createdAtUser, updatedAtUser, sessionID, createdAtSession, err := db.FuncCreateUser(req, sessions)
	if err != nil {
		return 0, err
	}
	if tools.EstNonVide(req.ProfilePictureID) {

		// A. PostgreSQL : Appel direct de la procédure stockée
		_, err := db.PostgresDB.Exec("CALL content.proc_update_media_owner($1, $2)", req.ProfilePictureID, userID)
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
		if err := db.Media.Update(filter, update); err != nil {
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

	if err := db.MongoCreateUser(req); err != nil {
		log.Printf("Erreur Mongo CreateUser: %v", err)
	}
	if err := db.MongoCreateSession(sessions); err != nil {
		log.Printf("Erreur Mongo CreateSession: %v", err)
	}
	// Enregistrement dans Redis
	if err := cache.RedisCreateUser(req); err != nil {
		log.Printf("Warning: Echec cache Redis User: %v", err)
	}
	if err := cache.RedisCreateSession(sessions); err != nil {
		log.Printf("Warning: Echec cache Redis Session: %v", err)
	}

	return userID, nil
}

func Login(
	input tools.LoginInput,
) (tools.UserRequest, tools.SessionsRequest, error) {
	var user tools.UserRequest
	var sessions tools.SessionsRequest
	var err error

	// Enregistrement dans Redis
	user, err = cache.RedisLoadUser(-1, "", input.Email, "")
	if err != nil {
		log.Printf("Erreur Redis LoadUser: %v", err)
	}
	if tools.EstNonVide(user) {
		if user.PasswordHash == input.PasswordHash {
			sessions, err = cache.RedisLoadSession(user.ID, input.DeviceToken)
			if err != nil {
				log.Printf("Erreur Redis LoadSession: %v", err)
			}
			if tools.EstNonVide(sessions) {
				return user, sessions, nil
			} else {
				// create session
				return user, sessions, nil
			}
		} else {
			return tools.UserRequest{}, tools.SessionsRequest{}, tools.ErrInvalidCredentials
		}
	} else {
		// Enregistrement dans MongoDB
		user, err = db.MongoLoadUser(-1, "", input.Email, "")
		if err != nil {
			log.Printf("Erreur Mongo CreateUser: %v", err)
		}
		if tools.EstNonVide(user) {
			if user.PasswordHash == input.PasswordHash {
				sessions, err = db.MongoLoadSession(user.ID, input.DeviceToken)
				if err != nil {
					log.Printf("Erreur Mongo CreateSession: %v", err)
				}
				if tools.EstNonVide(sessions) {
					return user, sessions, nil
				} else {
					// create session
					return user, sessions, nil
				}
			} else {
				return tools.UserRequest{}, tools.SessionsRequest{}, tools.ErrInvalidCredentials
			}
		} else {
			// Recherche dans PostgreSQL
			user, err = db.FuncLoadUser(-1, "", input.Email, "")
			if err != nil {
				return tools.UserRequest{}, tools.SessionsRequest{}, err
			}
			if tools.EstNonVide(user) {
				if user.PasswordHash == input.PasswordHash {
					sessions, err = db.MongoLoadSession(user.ID, input.DeviceToken)
					if err != nil {
						log.Printf("Erreur Mongo CreateSession: %v", err)
					}
					if tools.EstNonVide(sessions) {
						return user, sessions, nil
					} else {
						// create session
						return user, sessions, nil
					}
				} else {
					return tools.UserRequest{}, tools.SessionsRequest{}, tools.ErrInvalidCredentials
				}
			} else {
				return tools.UserRequest{}, tools.SessionsRequest{}, tools.ErrNotFound
			}
		}
	}
}

// startWebsocket
func StartWebsocket() {
	// Logique WS ici
}
