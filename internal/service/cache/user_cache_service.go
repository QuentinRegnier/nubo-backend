package cache

import (
	"context"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

func GetUserProfilePosts(ctx context.Context, userID int64, offset int64, limit int64) ([]domain.PostRequest, error) {
	if offset >= variables.MaxUserPostsElements {
		return getPostsFromMongoPaginated("user_id", userID, offset, limit)
	}

	// NOUVEAU : Utilisation de la constante normalisée
	key := fmt.Sprintf(variables.RedisKeyUserProfile, userID)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}

// AddPostToUserProfile ajoute un post au ZSET de l'utilisateur avec un score chronologique précis.
// Le score est le timestamp Unix (ms) de création du post.
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
