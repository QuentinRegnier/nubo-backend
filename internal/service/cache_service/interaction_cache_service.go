package cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : GESTION DE L'IDEMPOTENCE DES INTERACTIONS
// ############################################################################

// getIdempotencyCollection retourne le Set Redis L1 approprié selon le type de cible.
func getIdempotencyCollection(targetEntityType int) *redis.Collection {
	if targetEntityType == 1 {
		return redis.CommentLikesSet
	}
	return redis.PostLikesSet
}

// TryAddLikeIdempotency gère l'idempotence pour l'ajout de Likes de manière thread-safe (O(1)).
// Retourne true si l'ajout est un succès (l'utilisateur n'avait pas encore liké).
func TryAddLikeIdempotency(ctx context.Context, targetEntityType int, targetID int64, userID int64) bool {
	redisCollection := getIdempotencyCollection(targetEntityType)

	elementsAddedCount, errRedis := redisCollection.SAddCount(ctx, targetID, userID)

	// Si errRedis != nil, cela échouera (false), ce qui est le comportement de sécurité souhaité.
	return errRedis == nil && elementsAddedCount > 0
}

// TryRemoveLikeIdempotency gère la suppression d'idempotence pour les Likes (O(1)).
// Retourne true si le retrait est un succès (l'utilisateur avait bien liké).
func TryRemoveLikeIdempotency(ctx context.Context, targetEntityType int, targetID int64, userID int64) bool {
	redisCollection := getIdempotencyCollection(targetEntityType)

	elementsRemovedCount, errRedis := redisCollection.SRemCount(ctx, targetID, userID)

	return errRedis == nil && elementsRemovedCount > 0
}
