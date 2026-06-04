package post_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// DeletePost gère la rétractation d'un post (Purge L1, Purge LSH, Soft Delete Workers).
func DeletePost(ctx context.Context, input post_models.DeletePostInput) error {
	// ─────────────────────────────────────────────────────────────────────────
	// 1. VERIFICATION DROIT D'ACCÈS ET RÉCUPÉRATION DE L'OBJET COMPLET
	// ─────────────────────────────────────────────────────────────────────────
	post, err := security_service.LeftPost(ctx, input.PostID, input.UserID)
	if err != nil {
		return err
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. PURGE SYNCHRONE DES CACHES (Disparition instantanée)
	// ─────────────────────────────────────────────────────────────────────────
	// A. Suppression du Post en RAM
	_ = object_cache_service.DeletePostFromObjectCache(ctx, input.PostID)

	// B. Purge des Commentaires en RAM (ZSET + JSON L1)
	object_cache_service.PurgePostCommentsFromL1(ctx, input.PostID)

	// C. Suppression du seau LSH
	_ = feed_service.PurgePostVectors(ctx, input.PostID)

	// ─────────────────────────────────────────────────────────────────────────
	// 3. ENVOI AUX WORKERS POUR CASCADE BDD
	// ─────────────────────────────────────────────────────────────────────────
	return redis.EnqueueDB(ctx, post.ID, 0, redis.EntityPost, redis.ActionDelete, post, redis.TargetAll)
}
