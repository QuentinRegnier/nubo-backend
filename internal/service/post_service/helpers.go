package post_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// fetchPostsCascade gère la récupération L1 -> L2 -> L3 pour un batch d'IDs.
func fetchPostsCascade(ctx context.Context, ids []int64) map[int64]post_models.PostPayload {
	postsMap := make(map[int64]post_models.PostPayload)
	var missingFromL1 []int64

	// Étape 1 : Object Cache LFU (Redis)
	for _, id := range ids {
		if p, err := object_cache_service.GetPostFromObjectCache(ctx, id); err == nil {
			postsMap[id] = p
		} else {
			missingFromL1 = append(missingFromL1, id)
		}
	}

	if len(missingFromL1) == 0 {
		return postsMap // Tous les posts étaient en RAM, retour instantané
	}

	// Étape 2 : Cold Storage (MongoDB)
	var missingFromL2 []int64
	mongoPosts, errMongo := mongo.MongoLoadPosts(missingFromL1)
	if errMongo == nil {
		for _, p := range mongoPosts {
			postsMap[p.ID] = p
			_ = object_cache_service.SetPostInObjectCache(ctx, p) // Réhydratation L1
		}
	}

	// Identification de ce qu'il reste à trouver
	for _, id := range missingFromL1 {
		if _, exists := postsMap[id]; !exists {
			missingFromL2 = append(missingFromL2, id)
		}
	}

	if len(missingFromL2) == 0 {
		return postsMap
	}

	// Étape 3 : Source of Truth (PostgreSQL) via ta fonction paramétrée
	pgPosts, errPg := postgres.FuncLoadPosts(missingFromL2, len(missingFromL2), 0)
	if errPg == nil {
		for _, p := range pgPosts {
			postsMap[p.ID] = p

			// A. Réhydratation du cache haute performance L1 (Redis JSON) - Synchrone
			_ = object_cache_service.SetPostInObjectCache(ctx, p)

			// B. Réhydratation du stockage à froid L2 (MongoDB) - Asynchrone
			go func(post post_models.PostPayload) {
				bgCtx := context.Background()
				_ = redis.EnqueueDB(bgCtx, post.ID, post.UserID, redis.EntityPost, redis.ActionUpdate, post, redis.TargetMongo)
			}(p)
		}
	}

	return postsMap
}
