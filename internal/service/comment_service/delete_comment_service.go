package comment_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// ############################################################################
// # SERVICE : SUPPRESSION DE COMMENTAIRE (SOFT DELETE)
// ############################################################################

// DeleteComment gère la rétractation d'un commentaire (Purge L1, Soft Delete asynchrone et décrémentation du parent).
func DeleteComment(ctx context.Context, input comment_models.DeleteCommentInput) error {

	// ── ÉTAPE 1 : VÉRIFICATION DES DROITS (SÉCURITÉ) ────────────────────────

	// LeftComment gère déjà ses propres nubo_error (NotFound ou Forbidden), on retourne directement
	commentPayload, errSecurity := security_service.LeftComment(ctx, input.CommentID, input.UserID)
	if errSecurity != nil {
		return errSecurity
	}

	// ── ÉTAPE 2 : PURGE DU CACHE L1 ET PRÉPARATION DU SOFT DELETE ───────────

	_ = object_cache_service.DeleteCommentFromObjectCache(ctx, commentPayload.ID)
	_ = object_cache_service.RemoveCommentFromZSET(ctx, commentPayload.PostID, commentPayload.ID)

	// Décrémentation en Temps Réel du Post Parent
	if postPayload, errPost := object_cache_service.GetPostFromObjectCache(ctx, commentPayload.PostID); errPost == nil {
		postPayload.CommentCount -= 1
		if postPayload.CommentCount < 0 {
			postPayload.CommentCount = 0
		}

		_ = object_cache_service.SetPostInObjectCache(ctx, postPayload)
		cache_service.UpdatePostRecommendationScore(ctx, postPayload)
	}

	// Application du statut de suppression logicielle
	commentPayload.Visibility = -1

	// ── ÉTAPE 3 : DÉLÉGATION AUX WORKERS BATCH (WRITE-BEHIND) ───────────────

	// L'ActionDelete va ordonner aux workers (most_cache, mongo, postgres)
	// de répercuter le -1 sur le CommentCount du post parent en BDD.
	errQueue := redis.EnqueueDB(ctx, commentPayload.ID, commentPayload.PostID, redis.EntityComment, redis.ActionDelete, commentPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("comment_id", commentPayload.ID).Msg("Échec critique : Impossible d'enqueue la suppression du commentaire")
		return nubo_error.NewInternal()
	}

	return nil
}
