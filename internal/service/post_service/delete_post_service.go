package post_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
)

// DeletePost gère la rétractation d'un post (Purge L1, Purge LSH, Soft Delete Workers).
func DeletePost(ctx context.Context, input post_models.DeletePostInput) error {
	var post post_models.PostPayload
	var found bool

	// ─────────────────────────────────────────────────────────────────────────
	// 1. LECTURE CASCADE (On a besoin de l'objet pour les hashtags et la sécu)
	// ─────────────────────────────────────────────────────────────────────────
	if p, err := cache_service.GetPostFromObjectCache(ctx, input.PostID); err == nil {
		post = p
		found = true
	} else {
		mongoPosts, errMongo := mongo.MongoLoadPosts([]int64{input.PostID})
		if errMongo == nil && len(mongoPosts) > 0 {
			post = mongoPosts[0]
			found = true
		} else {
			pgPosts, errPg := postgres.FuncLoadPosts([]int64{input.PostID}, 1, 0)
			if errPg == nil && len(pgPosts) > 0 {
				post = pgPosts[0]
				found = true
			}
		}
	}

	if !found {
		return errors.New("not found")
	}

	// 🛡 SÉCURITÉ : Vérification du propriétaire
	if post.UserID != input.UserID {
		return errors.New("unauthorized")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. PURGE SYNCHRONE DES CACHES (Disparition instantanée pour les utilisateurs)
	// ─────────────────────────────────────────────────────────────────────────

	// A. Suppression de l'Object Cache L1
	_ = cache_service.DeletePostFromObjectCache(ctx, input.PostID)

	// B. Suppression du seau LSH et du Vecteur (Délégué au service dédié)
	_ = feed_service.PurgePostVectors(ctx, input.PostID)

	// ─────────────────────────────────────────────────────────────────────────
	// 3. ENVOI AUX WORKERS POUR SOFT-DELETE (BDD et MOST Cache)
	// ─────────────────────────────────────────────────────────────────────────

	// On passe l'objet `post` ENTIER dans le payload. C'est crucial pour que
	// `most_cache_worker.go` puisse lire `post.Hashtags` et nettoyer les bons ZSETs.
	return redis.EnqueueDB(ctx, post.ID, 0, redis.EntityPost, redis.ActionDelete, post, redis.TargetAll)
}
