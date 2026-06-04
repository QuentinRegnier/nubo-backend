package security_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

func LeftComment(ctx context.Context, commentID int64, userID int64) (comment_models.CommentPayload, error) {
	var comment comment_models.CommentPayload
	var found bool

	// ─────────────────────────────────────────────────────────────────────────
	// 1. CASCADE DE LECTURE (L1 -> L2 -> L3) POUR VÉRIFIER LES DROITS
	// ─────────────────────────────────────────────────────────────────────────

	// L1 : Object Cache LFU (Redis)
	if c, err := object_cache_service.GetCommentFromObjectCache(ctx, commentID); err == nil {
		comment = c
		found = true
	} else {
		// L2 : Cold Storage (MongoDB) - Propre et Typé !
		mongoComments, errMongo := mongo.MongoLoadComments([]int64{commentID})
		if errMongo == nil && len(mongoComments) > 0 {
			comment = mongoComments[0]
			found = true
		}

		// L3 : Base de Données (PostgreSQL)
		if !found {
			pgComment, errPg := postgres.FuncGetComment(ctx, commentID)
			if errPg == nil {
				comment = pgComment
				found = true
			}
		}
	}

	// Si introuvable ou déjà supprimé
	if !found || comment.Visibility == -1 {
		return comment_models.CommentPayload{}, errors.New("not found")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. VÉRIFICATION DE LA SÉCURITÉ (Droits d'auteur)
	// ─────────────────────────────────────────────────────────────────────────
	if comment.UserID != userID {
		return comment_models.CommentPayload{}, errors.New("unauthorized")
	}

	return comment, nil
}
