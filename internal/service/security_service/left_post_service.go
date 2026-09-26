package security_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// ############################################################################
// # SERVICE DE SÉCURITÉ : DROITS DE PROPRIÉTÉ D'UNE PUBLICATION
// ############################################################################

// LeftPost récupère une publication complète (L1 -> L2 -> L3) et vérifie
// que l'utilisateur appelant en est bien l'auteur légitime.
func LeftPost(ctx context.Context, postID int64, userID int64) (post_models.PostPayload, error) {
	var postPayload post_models.PostPayload
	var isPostFound bool

	// ── ÉTAPE 1 : TENTATIVE L1 (OBJECT CACHE - LFU) ─────────────────────────

	if cachedPost, errCache := object_cache_service.GetPostFromObjectCache(ctx, postID); errCache == nil && cachedPost.ID != 0 {
		postPayload = cachedPost
		isPostFound = true
	} else {

		// ── ÉTAPE 2 : TENTATIVE L2 (MONGODB WARM STORAGE) ───────────────────

		mongoPostsList, errMongo := mongo.MongoLoadPosts([]int64{postID})
		if errMongo == nil && len(mongoPostsList) > 0 {
			postPayload = mongoPostsList[0]
			isPostFound = true

			// AUTO-GUÉRISON L1 (Immédiat en RAM dans une goroutine pour libérer le thread)
			go func(p post_models.PostPayload) {
				backgroundCtx := context.Background()
				_ = object_cache_service.SetPostInObjectCache(backgroundCtx, p)
			}(postPayload)

		} else {

			// ── ÉTAPE 3 : FALLBACK ABSOLU L3 (POSTGRESQL COLD STORAGE) ──────

			pgPostsList, errPg := postgres.FuncLoadPosts([]int64{postID}, 1, 0)
			if errPg != nil {
				logger.Log.Error().Err(errPg).Int64("post_id", postID).Msg("Erreur L3 lors de la vérification de sécurité d'un post")
				return post_models.PostPayload{}, nubo_error.NewInternal()
			}

			if len(pgPostsList) > 0 {
				postPayload = pgPostsList[0]
				isPostFound = true

				// AUTO-GUÉRISON L1 (Immédiat en RAM)
				_ = object_cache_service.SetPostInObjectCache(ctx, postPayload)

				// AUTO-GUÉRISON L2 (Asynchrone via Worker Mongo)
				go func(p post_models.PostPayload) {
					backgroundCtx := context.Background()
					// PartitionKey = UserID pour le sharding des posts
					_ = redis.EnqueueDB(backgroundCtx, p.ID, p.UserID, redis.EntityPost, redis.ActionUpdate, p, redis.TargetMongo)
				}(postPayload)
			}
		}
	}

	// ── ÉTAPE 4 : VÉRIFICATION DES RÈGLES DE SÉCURITÉ ───────────────────────

	if !isPostFound {
		return post_models.PostPayload{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Publication introuvable ou supprimée.", nil)
	}

	if postPayload.UserID != userID {
		return post_models.PostPayload{}, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Vous n'êtes pas autorisé à réaliser cette action sur cette publication.", nil)
	}

	return postPayload, nil
}
