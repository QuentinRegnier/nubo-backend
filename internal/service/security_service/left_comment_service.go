package security_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

func LeftComment(ctx context.Context, commentID int64, userID int64) (comment_models.CommentPayload, error) {
	var comment comment_models.CommentPayload
	var found bool

	// 1. CASCADE DE LECTURE (L1 -> L2 -> L3) AVEC AUTO-GUÉRISON
	if c, err := object_cache_service.GetCommentFromObjectCache(ctx, commentID); err == nil {
		comment = c
		found = true
	} else {
		// TENTATIVE L2 (MongoDB)
		mongoComments, errMongo := mongo.MongoLoadComments([]int64{commentID})
		if errMongo == nil && len(mongoComments) > 0 {
			comment = mongoComments[0]
			found = true

			// ⬆️ PROMOTION L2 -> L1 (Immédiat en RAM)
			_ = object_cache_service.SetCommentInObjectCache(ctx, comment)
		} else {
			// TENTATIVE L3 (PostgreSQL)
			pgComment, errPg := postgres.FuncGetComment(ctx, commentID)
			if errPg == nil {
				comment = pgComment
				found = true

				// ⬆️ PROMOTION L3 -> L1 (Immédiat en RAM)
				_ = object_cache_service.SetCommentInObjectCache(ctx, comment)

				// ⬆️ PROMOTION L3 -> L2 (Asynchrone via Worker Mongo)
				go func(c comment_models.CommentPayload) {
					bgCtx := context.Background()
					// PartitionKey = PostID pour les commentaires
					_ = redis.EnqueueDB(bgCtx, c.ID, c.PostID, redis.EntityComment, redis.ActionUpdate, c, redis.TargetMongo)
				}(comment)
			}
		}
	}

	if !found || comment.Visibility == -1 {
		return comment_models.CommentPayload{}, nubo_error.NewNotFound("COMMENT_NOT_FOUND", "Commentaire introuvable ou supprimé.", nil)
	}

	// 2. VÉRIFICATION DE LA SÉCURITÉ (Droits d'auteur)
	if comment.UserID != userID {
		return comment_models.CommentPayload{}, nubo_error.NewForbidden("ACCESS_DENIED", "Vous n'êtes pas autorisé à réaliser cette action sur ce commentaire.", nil)
	}

	return comment, nil
}
