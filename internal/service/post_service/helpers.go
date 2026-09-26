package post_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// ############################################################################
// # UTILITAIRES : HYDRATATION ET AUTO-GUÉRISON (DOMAINE POST)
// ############################################################################

// fetchPostsCascade gère la récupération L1 -> L2 -> L3 pour un batch d'IDs,
// tout en assurant l'auto-guérison des couches supérieures si un cache miss survient.
func fetchPostsCascade(ctx context.Context, requestedPostIDs []int64) map[int64]post_models.PostPayload {

	resolvedPostsMap := make(map[int64]post_models.PostPayload)
	var postIDsMissingFromL1 []int64

	// ── ÉTAPE 1 : TENTATIVE L1 (RAM OBJECT CACHE) ───────────────────────────
	for _, postID := range requestedPostIDs {
		if postPayload, errCache := object_cache_service.GetPostFromObjectCache(ctx, postID); errCache == nil {
			resolvedPostsMap[postID] = postPayload
		} else {
			postIDsMissingFromL1 = append(postIDsMissingFromL1, postID)
		}
	}

	if len(postIDsMissingFromL1) == 0 {
		return resolvedPostsMap // Hit L1 à 100%, retour instantané
	}

	// ── ÉTAPE 2 : TENTATIVE L2 (MONGODB WARM STORAGE) ───────────────────────
	var postIDsMissingFromL2 []int64

	postsFromMongo, errMongo := mongo.MongoLoadPosts(postIDsMissingFromL1)
	if errMongo == nil && len(postsFromMongo) > 0 {
		for _, postPayload := range postsFromMongo {
			resolvedPostsMap[postPayload.ID] = postPayload

			// AUTO-GUÉRISON L1 (RAM)
			_ = object_cache_service.SetPostInObjectCache(ctx, postPayload)
		}
	}

	// Identification du reliquat après le passage L2
	for _, postID := range postIDsMissingFromL1 {
		if _, isResolved := resolvedPostsMap[postID]; !isResolved {
			postIDsMissingFromL2 = append(postIDsMissingFromL2, postID)
		}
	}

	if len(postIDsMissingFromL2) == 0 {
		return resolvedPostsMap
	}

	// ── ÉTAPE 3 : SOURCE DE VÉRITÉ L3 (POSTGRESQL COLD STORAGE) ─────────────

	postsFromPostgres, errPg := postgres.FuncLoadPosts(postIDsMissingFromL2, len(postIDsMissingFromL2), 0)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Msg("Erreur critique lors du Fetch L3 des posts manquants")
		return resolvedPostsMap // Retourne la map partielle pour limiter les dégâts
	}

	for _, postPayload := range postsFromPostgres {
		resolvedPostsMap[postPayload.ID] = postPayload

		// AUTO-GUÉRISON L1 (Synchrone en RAM)
		_ = object_cache_service.SetPostInObjectCache(ctx, postPayload)

		// AUTO-GUÉRISON L2 (Asynchrone vers Mongo)
		go func(p post_models.PostPayload) {
			bgCtx := context.Background()
			_ = redis.EnqueueDB(bgCtx, p.ID, p.UserID, redis.EntityPost, redis.ActionUpdate, p, redis.TargetMongo)
		}(postPayload)
	}

	return resolvedPostsMap
}
