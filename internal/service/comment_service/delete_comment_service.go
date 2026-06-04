package comment_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// DeleteComment gère la rétractation d'un commentaire (Purge L1, Soft Delete asynchrone et décrémentation).
func DeleteComment(ctx context.Context, input comment_models.DeleteCommentInput) error {
	// ─────────────────────────────────────────────────────────────────────────
	// 1. VERIFICATION DROIT D'ACCÈS ET RÉCUPÉRATION DE L'OBJET COMPLET (Nécessaire pour les étapes suivantes)
	// ─────────────────────────────────────────────────────────────────────────

	comment, err := security_service.LeftComment(ctx, input.CommentID, input.UserID)
	if err != nil {
		return err
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. PURGE DU CACHE L1 ET PRÉPARATION DU SOFT DELETE
	// ─────────────────────────────────────────────────────────────────────────

	// Disparition immédiate de la RAM pour les prochains lecteurs
	_ = object_cache_service.DeleteCommentFromObjectCache(ctx, comment.ID)
	_ = object_cache_service.RemoveCommentFromZSET(ctx, comment.PostID, comment.ID)

	comment.Visibility = -1

	// ─────────────────────────────────────────────────────────────────────────
	// 3. ENVOI AUX WORKERS POUR DÉCRÉMENTATION ET MISE À JOUR BDD
	// ─────────────────────────────────────────────────────────────────────────
	// L'ActionDelete va ordonner à most_cache_worker, mongo_batch et postgres_batch
	// de faire un "-1" sur le CommentCount du post parent.
	return redis.EnqueueDB(ctx, comment.ID, comment.PostID, redis.EntityComment, redis.ActionDelete, comment, redis.TargetAll)
}
