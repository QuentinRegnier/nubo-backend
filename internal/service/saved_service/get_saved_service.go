package saved_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DES PUBLICATIONS SAUVEGARDÉES
// ############################################################################

// GetSavedPosts récupère la liste paginée des favoris d'un utilisateur
// avec la garantie d'auto-guérison en cascade L1 -> L2 -> L3.
func GetSavedPosts(ctx context.Context, userID int64, limit int, offset int) ([]post_models.GetPostOutput, error) {
	var targetPostIDs []int64

	// ── ÉTAPE 1 : TENTATIVE L1 (RAM REDIS ZSET) ─────────────────────────────

	cachedPostIDs, errCache := object_cache_service.GetSavedPostIDs(ctx, userID, int64(offset), int64(limit))
	if errCache == nil && len(cachedPostIDs) > 0 {
		targetPostIDs = cachedPostIDs
	} else {

		// ── ÉTAPE 2 : FALLBACK L2 (MONGODB WARM STORAGE) ────────────────────
		savedPayloadsFromMongo, errMongo := mongo.MongoLoadSavedPosts(userID, int64(limit), int64(offset))

		if errMongo == nil && len(savedPayloadsFromMongo) > 0 {
			for _, savedPayload := range savedPayloadsFromMongo {
				targetPostIDs = append(targetPostIDs, savedPayload.PostID)

				// AUTO-GUÉRISON L1 (ZSET)
				_ = object_cache_service.AddSavedToZSET(ctx, userID, savedPayload.PostID, float64(savedPayload.CreatedAt))
			}
		} else {

			// ── ÉTAPE 3 : FALLBACK ABSOLU L3 (POSTGRESQL COLD STORAGE) ──────
			savedPayloadsFromPostgres, errPg := postgres.FuncLoadSavedPosts(ctx, userID, limit, offset)
			if errPg != nil {
				logger.Log.Error().Err(errPg).Int64("user_id", userID).Msg("Erreur L3 lors de la récupération des posts sauvegardés")
				return nil, nubo_error.NewInternal()
			}

			if len(savedPayloadsFromPostgres) > 0 {
				for _, savedPayload := range savedPayloadsFromPostgres {
					targetPostIDs = append(targetPostIDs, savedPayload.PostID)

					// AUTO-GUÉRISON L1 (Synchrone en RAM)
					_ = object_cache_service.AddSavedToZSET(ctx, userID, savedPayload.PostID, float64(savedPayload.CreatedAt))

					// AUTO-GUÉRISON L2 (Asynchrone via Workers)
					go func(payload saved_models.SavedPayload) {
						backgroundCtx := context.Background()
						// PartitionKey = UserID pour centraliser les favoris d'un même utilisateur
						_ = redis.EnqueueDB(backgroundCtx, payload.ID, payload.UserID, redis.EntitySaved, redis.ActionUpdate, payload, redis.TargetMongo)
					}(savedPayload)
				}
			}
		}
	}

	// ── ÉTAPE 4 : COUPE-CIRCUIT (AUCUN FAVORI) ──────────────────────────────

	if len(targetPostIDs) == 0 {
		return []post_models.GetPostOutput{}, nil
	}

	// ── ÉTAPE 5 : DÉLÉGATION AU POST_SERVICE POUR HYDRATATION MASSIVE ───────

	// On passe la liste d'IDs au pipeline unifié GetPosts qui va s'occuper :
	// - De vérifier les droits d'accès
	// - De filtrer les posts supprimés/privés
	// - De générer les URLs S3/MinIO signées (HMAC)
	// - D'hydrater les commentaires
	requestInput := post_models.GetPostInput{
		UserID:  userID,
		PostIDs: targetPostIDs,
	}

	hydratedPostsResults := post_service.GetPosts(ctx, requestInput)

	return hydratedPostsResults, nil
}
