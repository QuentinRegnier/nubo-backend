package security_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

func LeftPost(ctx context.Context, postID int64, userID int64) (post_models.PostPayload, error) {
	var post post_models.PostPayload
	var found bool

	// 1. CASCADE DE LECTURE (L1 -> L2 -> L3) AVEC AUTO-GUÉRISON
	if p, err := object_cache_service.GetPostFromObjectCache(ctx, postID); err == nil {
		post = p
		found = true
	} else {
		// TENTATIVE L2 (MongoDB)
		mongoPosts, errMongo := mongo.MongoLoadPosts([]int64{postID})
		if errMongo == nil && len(mongoPosts) > 0 {
			post = mongoPosts[0]
			found = true

			// PROMOTION L2 -> L1
			go func(p post_models.PostPayload) {
				_ = object_cache_service.SetPostInObjectCache(context.Background(), p)
			}(post)

		} else {
			// TENTATIVE L3 (PostgreSQL)
			pgPosts, errPg := postgres.FuncLoadPosts([]int64{postID}, 1, 0)
			if errPg == nil && len(pgPosts) > 0 {
				post = pgPosts[0]
				found = true

				// PROMOTION L3 -> L2 & L1
				go func(p post_models.PostPayload) {
					_ = mongo.MongoUpsertPost(p)
					_ = object_cache_service.SetPostInObjectCache(context.Background(), p)
				}(post)
			}
		}
	}

	if !found {
		return post_models.PostPayload{}, errors.New("not found")
	}

	// 2. CONTRÔLE D'AUTORISATION
	if post.UserID != userID {
		return post_models.PostPayload{}, errors.New("unauthorized")
	}

	return post, nil
}
