package service

import (
	"context"
	"fmt"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// AddPostToUserProfile ajoute un post au ZSET de l'utilisateur avec un score chronologique précis.
// Le score est le timestamp Unix (ms) de création du post.
func AddPostToUserProfile(ctx context.Context, userID int64, postID int64, createdAt int64) {
	key := fmt.Sprintf("user:posts:%d", userID)
	score := float64(createdAt)

	// Ajouter au ZSET via ton wrapper de repository
	if err := redis.ZAdd(ctx, key, score, postID); err != nil {
		return
	}

	// Éviction : on ne garde que les X derniers
	_ = redis.ZRemRangeByRank(ctx, key, 0, -(variables.MaxUserPostsElements + 1))
}

// InvalidateUserProfileCache détruit le ZSET d'un utilisateur pour forcer une réhydratation
// complète depuis PostgreSQL au prochain appel (Cache Busting).
func InvalidateUserProfileCache(ctx context.Context, userID int64) {
	key := fmt.Sprintf("user:posts:%d", userID)

	// Utilisation directe du client de l'infrastructure pour un DEL massif en O(1)
	_ = redisgo.Rdb.Del(ctx, key).Err()
}
