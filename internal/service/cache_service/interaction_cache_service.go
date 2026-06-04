package cache_service

import (
	"context"
	"fmt"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
)

// getLikeKey génère dynamiquement la clé selon le type de cible
func getLikeKey(targetType int, targetID int64) string {
	prefix := "post"
	if targetType == 1 {
		prefix = "comment"
	}
	return fmt.Sprintf("%s:likes_set:%d", prefix, targetID)
}

// TryAddLikeIdempotency gère l'idempotence pour posts et commentaires.
func TryAddLikeIdempotency(ctx context.Context, targetType int, targetID int64, userID int64) bool {
	setKey := getLikeKey(targetType, targetID)
	added, err := redisgo.Rdb.SAdd(ctx, setKey, userID).Result()
	return err == nil && added > 0
}

// TryRemoveLikeIdempotency gère la suppression d'idempotence pour posts et commentaires.
func TryRemoveLikeIdempotency(ctx context.Context, targetType int, targetID int64, userID int64) bool {
	setKey := getLikeKey(targetType, targetID)
	removed, err := redisgo.Rdb.SRem(ctx, setKey, userID).Result()
	return err == nil && removed > 0
}
