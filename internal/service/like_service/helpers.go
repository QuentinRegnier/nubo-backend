package like_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// ############################################################################
// # UTILITAIRES D'HYDRATATION : DOMAINE LIKES
// ############################################################################

// getCommentCascade est le fallback local ultra-rapide pour hydrater l'objet métier.
// Utilisé avant de traiter un like de commentaire pour s'assurer que le parent n'a pas été supprimé.
func getCommentCascade(ctx context.Context, commentID int64) (comment_models.CommentPayload, error) {

	// ── ÉTAPE 1 : TENTATIVE L1 (RAM OBJECT CACHE) ───────────────────────────
	if commentPayload, errCache := object_cache_service.GetCommentFromObjectCache(ctx, commentID); errCache == nil {
		return commentPayload, nil
	}

	// ── ÉTAPE 2 : TENTATIVE L2 (MONGODB WARM STORAGE) ───────────────────────
	commentsFromMongo, errMongo := mongo.MongoLoadComments([]int64{commentID})
	if errMongo == nil && len(commentsFromMongo) > 0 {
		_ = object_cache_service.SetCommentInObjectCache(ctx, commentsFromMongo[0])
		return commentsFromMongo[0], nil
	}

	// ── ÉTAPE 3 : TENTATIVE L3 (POSTGRESQL COLD STORAGE) ────────────────────
	commentFromPostgres, errPg := postgres.FuncGetComment(ctx, commentID)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Int64("comment_id", commentID).Msg("Erreur L3 lors de la récupération du commentaire en cascade")
		return comment_models.CommentPayload{}, nubo_error.NewInternal()
	}

	if commentFromPostgres.ID != 0 {
		// AUTO-GUÉRISON L1 (Synchrone)
		_ = object_cache_service.SetCommentInObjectCache(ctx, commentFromPostgres)

		// AUTO-GUÉRISON L2 (Asynchrone via Worker Mongo)
		go func(c comment_models.CommentPayload) {
			bgCtx := context.Background()
			_ = redis.EnqueueDB(bgCtx, c.ID, c.PostID, redis.EntityComment, redis.ActionUpdate, c, redis.TargetMongo)
		}(commentFromPostgres)

		return commentFromPostgres, nil
	}

	return comment_models.CommentPayload{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Commentaire introuvable ou supprimé.", nil)
}
