package cache_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

func GetUserProfilePosts(ctx context.Context, userID int64, offset int64, limit int64) ([]post_models.PostPayload, error) {
	if offset >= variables.MaxUserPostsElements {
		return getPostsFromMongoPaginated("user_id", userID, offset, limit)
	}

	// NOUVEAU : Utilisation de la constante normalisée
	key := fmt.Sprintf(variables.RedisKeyUserProfile, userID)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}

// AddPostToUserProfile ajoute un post_service au ZSET de l'utilisateur avec un score chronologique précis.
// Le score est le timestamp Unix (ms) de création du post_service.
func AddPostToUserProfile(ctx context.Context, userID int64, postID int64, createdAt int64) {
	// NOUVEAU : Utilisation de la constante normalisée
	key := fmt.Sprintf(variables.RedisKeyUserProfile, userID)
	score := float64(createdAt)

	// NOUVEAU : Insertion et plafonnement 100% atomique via Lua
	// Remplace le double appel ZADD + ZREMRANGEBYRANK qui causait des race conditions.
	_ = redis.ZAddWithCap(ctx, key, score, postID, variables.MaxUserPostsElements)
}

// InvalidateUserProfileCache détruit le ZSET d'un utilisateur pour forcer une réhydratation
// complète depuis PostgreSQL au prochain appel (Cache Busting).
func InvalidateUserProfileCache(ctx context.Context, userID int64) {
	// NOUVEAU : Utilisation de la constante normalisée
	key := fmt.Sprintf(variables.RedisKeyUserProfile, userID)

	// Utilisation directe du client de l'infrastructure pour un DEL massif en O(1)
	_ = redisgo.Rdb.Del(ctx, key).Err()
}

// Helper local pour protéger l'API des blocages si Redis ne répond pas
func getShortCtx(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, 2*time.Second)
}

// ---------------- USER FULL ----------------

// SetUserFullInCache sauvegarde l'utilisateur et crée des index légers (Pointeurs)
func SetUserFullInCache(ctx context.Context, u models.UserRequest) error {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	if err := redis.Users.SetObject(c, u.ID, u); err != nil {
		return err
	}

	pipe := redisgo.Rdb.Pipeline()
	if u.Email != "" {
		pipe.Set(c, fmt.Sprintf("idx:user:email:%s", u.Email), u.ID, redis.Users.DefaultTTL)
	}
	if u.Username != "" {
		pipe.Set(c, fmt.Sprintf("idx:user:username:%s", u.Username), u.ID, redis.Users.DefaultTTL)
	}
	if u.Phone != "" {
		pipe.Set(c, fmt.Sprintf("idx:user:phone:%s", u.Phone), u.ID, redis.Users.DefaultTTL)
	}

	_, err := pipe.Exec(c)
	return err
}

// LoadUserFullFromCache charge un utilisateur par ID, Username, Email ou Phone
func LoadUserFullFromCache(ctx context.Context, id int64, username string, email string, phone string) (models.UserRequest, error) {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	var targetID int64 = id

	if targetID <= 0 {
		var key string
		if email != "" {
			key = fmt.Sprintf("idx:user:email:%s", email)
		} else if username != "" {
			key = fmt.Sprintf("idx:user:username:%s", username)
		} else if phone != "" {
			key = fmt.Sprintf("idx:user:phone:%s", phone)
		} else {
			return models.UserRequest{}, fmt.Errorf("aucun critère de recherche")
		}

		val, err := redisgo.Rdb.Get(c, key).Int64()
		if err != nil {
			return models.UserRequest{}, fmt.Errorf("utilisateur introuvable dans redis (index miss)")
		}
		targetID = val
	}

	var u models.UserRequest
	if err := redis.Users.GetObject(c, targetID, &u); err != nil {
		return models.UserRequest{}, err
	}

	return u, nil
}

// ---------------- SESSION ----------------

// SetSessionInCache sauvegarde la session et son index de recherche
func SetSessionInCache(ctx context.Context, s models.SessionsRequest) error {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	if err := redis.Sessions.SetObject(c, s.ID, s); err != nil {
		return err
	}

	if s.UserID != 0 && s.DeviceToken != "" {
		idxKey := fmt.Sprintf("idx:session:%d:%s", s.UserID, s.DeviceToken)
		return redisgo.Rdb.Set(c, idxKey, s.ID, redis.Sessions.DefaultTTL).Err()
	}

	return nil
}

// LoadSessionFromCache charge une session (CurrentSecret conservé dans la signature pour compatibilité).
func LoadSessionFromCache(ctx context.Context, userID int64, deviceToken string, masterToken string, currentSecret string) (models.SessionsRequest, error) {
	c, cancel := getShortCtx(ctx)
	defer cancel()

	var targetID int64

	if userID != -1 && deviceToken != "" {
		idxKey := fmt.Sprintf("idx:session:%d:%s", userID, deviceToken)
		val, err := redisgo.Rdb.Get(c, idxKey).Int64()
		if err == nil {
			targetID = val
		}
	}

	if targetID == 0 {
		return models.SessionsRequest{}, fmt.Errorf("session introuvable dans redis (index miss)")
	}

	var s models.SessionsRequest
	if err := redis.Sessions.GetObject(c, targetID, &s); err != nil {
		return models.SessionsRequest{}, err
	}

	if masterToken != "" && s.MasterToken != masterToken {
		return models.SessionsRequest{}, fmt.Errorf("master token mismatch")
	}

	return s, nil
}
