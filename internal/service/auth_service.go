package service

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/cuckoo"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	postgresgo "github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// addImage gère l'upload (S3/Disque)
func AddImage(rawBinaryData string, name string) int {
	reader := strings.NewReader(rawBinaryData)
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
	// (Le reste de CreateUser est inchangé...)
	if pkg.EstNonVide(req.ProfilePictureID) {
		_, err := postgres.PostgresDB.Exec("CALL content.proc_update_media_owner($1, $2)", req.ProfilePictureID, userID)
		if err != nil {
			log.Printf("⚠️ Erreur liaison avatar Postgres (MediaID: %v) : %v", req.ProfilePictureID, err)
		}
		filter := map[string]any{"id": req.ProfilePictureID}
		update := map[string]any{"owner_id": userID}
		if err := mongo.Media.Update(filter, update); err != nil {
			log.Printf("⚠️ Erreur liaison avatar Mongo (MediaID: %v) : %v", req.ProfilePictureID, err)
		} else {
			log.Printf("✅ Avatar lié avec succès (PG + Mongo) à l'utilisateur %v", userID)
		}
	}

	req.ID = userID
	req.CreatedAt = createdAtUser
	req.UpdatedAt = updatedAtUser
	sessions.ID = sessionID
	sessions.UserID = userID
	sessions.CreatedAt = createdAtSession
	sessions.MasterToken, err = pkg.GenerateToken(req.ID, sessions.DeviceToken, variables.MasterTokenExpirationSeconds)
	sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)

	if err := mongo.MongoCreateUser(req); err != nil {
		log.Printf("Erreur Mongo CreateUser: %v", err)
	}
	if err := mongo.MongoCreateSession(sessions); err != nil {
		log.Printf("Erreur Mongo CreateSession: %v", err)
	}
	if err := redis.RedisCreateUser(req); err != nil {
		log.Printf("Warning: Echec cache Redis User: %v", err)
	}
	if err := redis.RedisCreateSession(sessions); err != nil {
		log.Printf("Warning: Echec cache Redis Session: %v", err)
	}

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

	return userID, nil
}

func Login(
	input domain.LoginInput,
) (domain.UserRequest, domain.SessionsRequest, error) {
	fmt.Printf("\n🚀 SERVICE LOGIN APPELÉ pour l'email : [%s]\n", input.Email)

	var user domain.UserRequest
	var sessions domain.SessionsRequest
	var err error

	// 1. Redis
	user, err = redis.RedisLoadUser(-1, "", input.Email, "")
	if err != nil {
		fmt.Printf("🔸 Redis: erreur (%v)\n", err)
	}

	// CORRECTION ICI : On vérifie l'ID, pas EstNonVide
	if user.ID == 0 {
		fmt.Println("🔸 Redis: User non trouvé, passage à Mongo...")

		// 2. Mongo
		user, err = mongo.MongoLoadUser(-1, "", input.Email, "")
		if err != nil {
			fmt.Printf("🔸 Mongo: erreur (%v)\n", err)
		}

		// CORRECTION ICI
		if user.ID == 0 {
			fmt.Println("🔸 Mongo: User non trouvé, passage à Postgres...")

			// 3. Postgres
			user, err = postgresgo.FuncLoadUser(-1, "", input.Email, "")
			if err != nil {
				fmt.Printf("❌ ERREUR CRITIQUE APPEL POSTGRES : %v\n", err)
				return domain.UserRequest{}, domain.SessionsRequest{}, err
			}

			// CORRECTION ICI
			if user.ID == 0 {
				fmt.Printf("❌ ERREUR : Utilisateur introuvable dans AUCUNE base pour l'email '%s'\n", input.Email)
				return domain.UserRequest{}, domain.SessionsRequest{}, domain.ErrNotFound
			}
		}
	}

	fmt.Println("✅ UTILISATEUR TROUVÉ ! Analyse du mot de passe...")
	fmt.Printf("👉 INPUT : '%s' (Len: %d)\n", input.PasswordHash, len(input.PasswordHash))
	fmt.Printf("👉 DB    : '%s' (Len: %d)\n", user.PasswordHash, len(user.PasswordHash))

	// Comparaison Mot de passe
	// TRIM pour éviter les problèmes d'espaces si la BDD est sale
	if strings.TrimSpace(user.PasswordHash) != strings.TrimSpace(input.PasswordHash) {
		fmt.Println("🔒 MOT DE PASSE INCORRECT (Mismatch)")
		return domain.UserRequest{}, domain.SessionsRequest{}, domain.ErrInvalidCredentials
	} else if user.Desactivated == true || user.Banned == true {
		if user.Desactivated == true && user.Banned == false {
			return domain.UserRequest{}, domain.SessionsRequest{}, domain.ErrDesactivated
		} else {
			return domain.UserRequest{}, domain.SessionsRequest{}, domain.ErrBanned
		}
	} else {
		fmt.Println("🔓 MOT DE PASSE VALIDE ! Chargement session...")

		sessions, err = redis.RedisLoadSession(user.ID, input.DeviceToken, "", "")
		if sessions.ID != 0 && sessions.UserID == user.ID && sessions.DeviceToken == input.DeviceToken {
			sessions.DeviceInfo = input.DeviceInfo
			if pkg.Exists(sessions.IPHistory, input.IPAddress[0]) == false && len(input.IPAddress) > 0 {
				sessions.IPHistory = append(sessions.IPHistory, input.IPAddress[0])
			}
			sessions.ExpiresAt = time.Now().Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)
			sessions.MasterToken, err = pkg.GenerateToken(user.ID, input.DeviceToken, variables.MasterTokenExpirationSeconds)
			sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)
			sessions.LastSecret = sessions.DeviceToken
			sessions.LastJWT = ""
			sessions.ToleranceTime = time.Time{}
			if err != nil {
				return domain.UserRequest{}, domain.SessionsRequest{}, err
			}
			err = redis.RedisUpdateSession(sessions)
			if err != nil {
				return domain.UserRequest{}, domain.SessionsRequest{}, err
			}
			go func(sess domain.SessionsRequest) {
				errMongo := mongo.MongoUpdateSession(sess)
				if errMongo != nil {
					log.Printf("Erreur Mongo UpdateSession: %v", errMongo)
				}
			}(sessions)
			go func(sess domain.SessionsRequest) {
				errPg := postgresgo.ProcUpdateSession(sess.ID, sess.MasterToken, sess.DeviceInfo, sess.DeviceToken, sess.IPHistory, sess.ExpiresAt)
				if errPg != nil {
					log.Printf("Erreur Postgres UpdateSession: %v", errPg)
				}
			}(sessions)
			return user, sessions, nil
		}
		sessions, err = mongo.MongoLoadSession(user.ID, input.DeviceToken, "", "")
		if sessions.ID != 0 && sessions.UserID == user.ID && sessions.DeviceToken == input.DeviceToken {
			sessions.DeviceInfo = input.DeviceInfo
			if pkg.Exists(sessions.IPHistory, input.IPAddress[0]) == false && len(input.IPAddress) > 0 {
				sessions.IPHistory = append(sessions.IPHistory, input.IPAddress[0])
			}
			sessions.ExpiresAt = time.Now().Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)
			sessions.MasterToken, err = pkg.GenerateToken(user.ID, input.DeviceToken, variables.MasterTokenExpirationSeconds)
			sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)
			sessions.LastSecret = sessions.DeviceToken
			sessions.LastJWT = ""
			sessions.ToleranceTime = time.Time{}
			if err != nil {
				return domain.UserRequest{}, domain.SessionsRequest{}, err
			}
			err = postgresgo.ProcUpdateSession(sessions.ID, sessions.MasterToken, sessions.DeviceInfo, sessions.DeviceToken, sessions.IPHistory, sessions.ExpiresAt)
			if err != nil {
				return domain.UserRequest{}, domain.SessionsRequest{}, err
			}
			_ = redis.RedisCreateSession(sessions)
			go func(sess domain.SessionsRequest) {
				errMongo := mongo.MongoUpdateSession(sess)
				if errMongo != nil {
					log.Printf("Erreur Mongo UpdateSession: %v", errMongo)
				}
			}(sessions)
			go func(sess domain.SessionsRequest) {
				errPg := postgresgo.ProcUpdateSession(sess.ID, sess.MasterToken, sess.DeviceInfo, sess.DeviceToken, sess.IPHistory, sess.ExpiresAt)
				if errPg != nil {
					log.Printf("Erreur Postgres UpdateSession: %v", errPg)
				}
			}(sessions)
			return user, sessions, nil
		}

		sessions, err = postgresgo.FuncLoadSession(-1, user.ID, input.DeviceToken, "")
		if err != nil {
			return domain.UserRequest{}, domain.SessionsRequest{}, err
		}
		if sessions.ID != 0 && sessions.UserID == user.ID && sessions.DeviceToken == input.DeviceToken {
			// Modifier la session pour mettre à jour les infos
			sessions.DeviceInfo = input.DeviceInfo
			if pkg.Exists(sessions.IPHistory, input.IPAddress[0]) == false && len(input.IPAddress) > 0 {
				sessions.IPHistory = append(sessions.IPHistory, input.IPAddress[0])
			}
			sessions.ExpiresAt = time.Now().Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)
			sessions.MasterToken, err = pkg.GenerateToken(user.ID, input.DeviceToken, variables.MasterTokenExpirationSeconds)
			sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)
			sessions.LastSecret = sessions.DeviceToken
			sessions.LastJWT = ""
			sessions.ToleranceTime = time.Time{}
			if err != nil {
				return domain.UserRequest{}, domain.SessionsRequest{}, err
			}
			_ = redis.RedisCreateSession(sessions)
			go func(sess domain.SessionsRequest) {
				_ = mongo.MongoCreateSession(sessions)
			}(sessions)
			go func(sess domain.SessionsRequest) {
				errPg := postgresgo.ProcUpdateSession(sess.ID, sess.MasterToken, sess.DeviceInfo, sess.DeviceToken, sess.IPHistory, sess.ExpiresAt)
				if errPg != nil {
					log.Printf("Erreur Postgres UpdateSession: %v", errPg)
				}
			}(sessions)
			return user, sessions, nil
		}
		// Creer une session
		sessions.MasterToken, err = pkg.GenerateToken(user.ID, input.DeviceToken, variables.MasterTokenExpirationSeconds)
		sessions.DeviceInfo = input.DeviceInfo
		sessions.DeviceToken = input.DeviceToken
		sessions.CurrentSecret = security.DeriveNextSecret(sessions.DeviceToken, sessions.MasterToken, sessions.MasterToken, sessions.DeviceToken)
		sessions.LastSecret = sessions.DeviceToken
		sessions.LastJWT = ""
		sessions.ToleranceTime = time.Time{}
		if len(input.IPAddress) > 0 {
			sessions.IPHistory = []string{input.IPAddress[0]}
		} else {
			return domain.UserRequest{}, domain.SessionsRequest{}, domain.ErrInvalidIPAddress
		}
		sessions.ExpiresAt = time.Now().Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)
		sessionID, createdAtSession, err := postgresgo.FuncCreateSession(user.ID, sessions.MasterToken, sessions.DeviceInfo, sessions.DeviceToken, sessions.IPHistory, sessions.ExpiresAt)
		if err != nil {
			return domain.UserRequest{}, domain.SessionsRequest{}, err
		}
		sessions.ID = sessionID
		sessions.UserID = user.ID
		sessions.CreatedAt = createdAtSession

		err = mongo.MongoCreateSession(sessions)
		if err != nil {
			log.Printf("Erreur Mongo CreateSession lors du login: %v", err)
		}
		err = redis.RedisCreateSession(sessions)
		if err != nil {
			log.Printf("Warning: Echec cache Redis Session lors du login: %v", err)
		}

		return user, sessions, nil
	}
}

// startWebsocket
func StartWebsocket() {

}
