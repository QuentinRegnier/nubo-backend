package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
)

// Helper pour le contexte (timeout court pour ne pas bloquer l'API)
func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

// ---------------- USER ----------------

// RedisCreateUser sauvegarde l'utilisateur et crée des index légers (Pointeurs)
func RedisCreateUser(u domain.UserRequest) error {
	ctx, cancel := getCtx()
	defer cancel()

	// 1. Sauvegarde de l'objet principal (JSON)
	if err := Users.SetObject(ctx, u.ID, u); err != nil {
		return err
	}

	// 2. Création des Index "Pointeurs" (Pour retrouver l'ID via Email/Username/Phone)
	// Clé: "idx:user:email:jean@test.com" -> Valeur: "18293..."
	// On utilise le même TTL que la collection
	pipe := redisgo.Rdb.Pipeline()

	if u.Email != "" {
		pipe.Set(ctx, fmt.Sprintf("idx:user:email:%s", u.Email), u.ID, Users.DefaultTTL)
	}
	if u.Username != "" {
		pipe.Set(ctx, fmt.Sprintf("idx:user:username:%s", u.Username), u.ID, Users.DefaultTTL)
	}
	if u.Phone != "" {
		pipe.Set(ctx, fmt.Sprintf("idx:user:phone:%s", u.Phone), u.ID, Users.DefaultTTL)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// RedisLoadUser charge un utilisateur par ID, Username, Email ou Phone
func RedisLoadUser(id int64, username string, email string, phone string) (domain.UserRequest, error) {
	ctx, cancel := getCtx()
	defer cancel()

	var targetID int64 = id

	// 1. Si on n'a pas l'ID, on cherche dans les index pointeurs
	if targetID <= 0 {
		var key string
		if email != "" {
			key = fmt.Sprintf("idx:user:email:%s", email)
		} else if username != "" {
			key = fmt.Sprintf("idx:user:username:%s", username)
		} else if phone != "" {
			key = fmt.Sprintf("idx:user:phone:%s", phone)
		} else {
			return domain.UserRequest{}, fmt.Errorf("aucun critère de recherche")
		}

		// Récupération de l'ID depuis l'index
		val, err := redisgo.Rdb.Get(ctx, key).Int64()
		if err != nil {
			return domain.UserRequest{}, fmt.Errorf("utilisateur introuvable dans redis (index miss)")
		}
		targetID = val
	}

	// 2. Récupération de l'objet complet
	var u domain.UserRequest
	if err := Users.GetObject(ctx, targetID, &u); err != nil {
		return domain.UserRequest{}, err
	}

	return u, nil
}

// ---------------- SESSION ----------------

// RedisCreateSession sauvegarde la session et son index de recherche
func RedisCreateSession(s domain.SessionsRequest) error {
	ctx, cancel := getCtx()
	defer cancel()

	// 1. Objet Principal
	if err := Sessions.SetObject(ctx, s.ID, s); err != nil {
		return err
	}

	// 2. Index de recherche (UserID + DeviceToken -> SessionID)
	// Utile pour retrouver la session lors du login ou middleware
	if s.UserID != 0 && s.DeviceToken != "" {
		idxKey := fmt.Sprintf("idx:session:%d:%s", s.UserID, s.DeviceToken)
		return redisgo.Rdb.Set(ctx, idxKey, s.ID, Sessions.DefaultTTL).Err()
	}

	return nil
}

// RedisUpdateSession est identique au Create dans un Object Store (Overwrite complet)
func RedisUpdateSession(s domain.SessionsRequest) error {
	return RedisCreateSession(s)
}

// RedisLoadSession charge une session.
// Note : MasterToken et CurrentSecret ne sont plus indexés car rarement utilisés pour la recherche primaire.
func RedisLoadSession(userID int64, deviceToken string, masterToken string, currentSecret string) (domain.SessionsRequest, error) {
	ctx, cancel := getCtx()
	defer cancel()

	var targetID int64

	// 1. Recherche de l'ID de session via l'index (UserID + DeviceToken)
	if userID != -1 && deviceToken != "" {
		idxKey := fmt.Sprintf("idx:session:%d:%s", userID, deviceToken)
		val, err := redisgo.Rdb.Get(ctx, idxKey).Int64()
		if err == nil {
			targetID = val
		}
	}

	// (Optionnel) Si on voulait chercher par MasterToken, il faudrait créer un index pour ça.
	// Pour l'instant, on assume que le flux principal passe par UserID+DeviceToken.

	if targetID == 0 {
		return domain.SessionsRequest{}, fmt.Errorf("session introuvable dans redis (index miss)")
	}

	// 2. Chargement de l'objet
	var s domain.SessionsRequest
	if err := Sessions.GetObject(ctx, targetID, &s); err != nil {
		return domain.SessionsRequest{}, err
	}

	// Petit check de cohérence optionnel
	if masterToken != "" && s.MasterToken != masterToken {
		return domain.SessionsRequest{}, fmt.Errorf("master token mismatch")
	}

	return s, nil
}

// ---------------- CONTENU (Simple Key-Value) ----------------

func RedisCreatePost(p domain.PostRequest) error {
	ctx, cancel := getCtx()
	defer cancel()
	return Posts.SetObject(ctx, p.ID, p)
}

func RedisCreateMedia(m domain.MediaRequest) error {
	ctx, cancel := getCtx()
	defer cancel()
	return Media.SetObject(ctx, m.ID, m)
}
