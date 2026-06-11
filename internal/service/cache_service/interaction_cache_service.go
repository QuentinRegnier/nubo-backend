package cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// getIdempotencyCollection retourne la bonne Collection L1 selon le type de cible
func getIdempotencyCollection(targetType int) *redis.Collection {
	if targetType == 1 {
		return redis.CommentLikesSet
	}
	return redis.PostLikesSet
}

// TryAddLikeIdempotency gère l'idempotence pour posts et commentaires de manière thread-safe.
func TryAddLikeIdempotency(ctx context.Context, targetType int, targetID int64, userID int64) bool {
	col := getIdempotencyCollection(targetType)
	added, err := col.SAddCount(ctx, targetID, userID)
	return err == nil && added > 0
}

// TryRemoveLikeIdempotency gère la suppression d'idempotence pour posts et commentaires.
func TryRemoveLikeIdempotency(ctx context.Context, targetType int, targetID int64, userID int64) bool {
	col := getIdempotencyCollection(targetType)
	removed, err := col.SRemCount(ctx, targetID, userID)
	return err == nil && removed > 0
}
