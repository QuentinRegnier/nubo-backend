package like_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetPostLikes récupère les abonnés ayant liké un post en appliquant les règles de visibilité.
func GetPostLikes(ctx context.Context, input post_models.GetPostLikesInput) (post_models.GetPostLikesOutput, error) {

	// ─────────────────────────────────────────────────────────────────────────
	// 1. SÉCURITÉ : VÉRIFICATION DES DROITS D'ACCÈS AU POST (L1 -> L2 -> L3)
	// ─────────────────────────────────────────────────────────────────────────
	var post post_models.PostPayload
	var found bool

	// Lecture ultra-rapide du post pour checker les droits
	if p, err := object_cache_service.GetPostFromObjectCache(ctx, input.PostID); err == nil {
		post = p
		found = true
	} else {
		// Fallback BDD si le post n'est plus en RAM
		pgPosts, errPg := postgres.FuncLoadPosts([]int64{input.PostID}, 1, 0)
		if errPg == nil && len(pgPosts) > 0 {
			post = pgPosts[0]
			found = true
		}
	}

	if !found || post.Visibility == -1 {
		return post_models.GetPostLikesOutput{}, errors.New("not found")
	}

	// Matrice de Confidentialité
	if post.UserID != input.CallerID {
		relationState := cache_service.RelationValue(ctx, post.UserID, input.CallerID)
		if relationState == -1 {
			return post_models.GetPostLikesOutput{}, errors.New("banned")
		}
		if post.Visibility == 1 && relationState < 1 { // Abonnés
			return post_models.GetPostLikesOutput{}, errors.New("forbidden")
		}
		if post.Visibility == 2 && relationState != 2 { // Amis
			return post_models.GetPostLikesOutput{}, errors.New("forbidden")
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. RÉCUPÉRATION DES LIKES (L2 Mongo -> L3 Postgres)
	// ─────────────────────────────────────────────────────────────────────────
	var userIDs []int64

	// On tente Mongo (L2) d'abord
	userIDs, errMongo := mongo.MongoGetPostLikes(input.PostID, input.Limit, input.Offset)

	// Si Mongo échoue ou ne renvoie rien (Cache miss), on tape Postgres (L3)
	if errMongo != nil || len(userIDs) == 0 {
		userIDsPg, errPg := postgres.FuncGetPostLikes(ctx, input.PostID, input.Limit, input.Offset)
		if errPg == nil {
			userIDs = userIDsPg
		}
	}

	// Si aucune donnée, on s'assure de renvoyer un tableau vide plutôt que 'null' en JSON
	if userIDs == nil {
		userIDs = make([]int64, 0)
	}

	return post_models.GetPostLikesOutput{
		PostID:  input.PostID,
		UserIDs: userIDs,
	}, nil
}
