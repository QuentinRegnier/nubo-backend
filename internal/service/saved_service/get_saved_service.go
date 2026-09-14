package saved_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/post_service"
)

// GetSavedPosts récupère la liste paginée des favoris d'un utilisateur avec garantie L1->L2->L3
func GetSavedPosts(ctx context.Context, userID int64, limit int, offset int) []post_models.GetPostOutput {
	var postIDs []int64

	// 1. TENTATIVE L1 (Redis ZSET)
	idsFromL1, err := object_cache_service.GetSavedPostIDs(ctx, userID, int64(offset), int64(limit))
	if err == nil && len(idsFromL1) > 0 {
		postIDs = idsFromL1
	} else {
		// 2. FALLBACK L2 (MongoDB)
		savedsL2, errL2 := mongo.MongoLoadSavedPosts(userID, int64(limit), int64(offset))
		if errL2 == nil && len(savedsL2) > 0 {
			for _, s := range savedsL2 {
				postIDs = append(postIDs, s.PostID)
				// Auto-guérison L1
				_ = object_cache_service.AddSavedToZSET(ctx, userID, s.PostID, float64(s.CreatedAt.UnixMilli()))
			}
		} else {
			// 3. FALLBACK ABSOLU L3 (PostgreSQL)
			savedsL3, errL3 := postgres.FuncLoadSavedPosts(ctx, userID, limit, offset)
			if errL3 == nil && len(savedsL3) > 0 {
				for _, s := range savedsL3 {
					postIDs = append(postIDs, s.PostID)

					// Auto-guérison L1 (Synchrone)
					_ = object_cache_service.AddSavedToZSET(ctx, userID, s.PostID, float64(s.CreatedAt.UnixMilli()))

					// Auto-guérison L2 (Asynchrone via les workers)
					go func(saved saved_models.SavedPayload) {
						bgCtx := context.Background()
						// On utilise le userID comme partitionKey pour centraliser les favoris d'un user
						_ = redis.EnqueueDB(bgCtx, saved.ID, saved.UserID, redis.EntitySaved, redis.ActionUpdate, saved, redis.TargetMongo)
					}(s)
				}
			}
		}
	}

	// Coupe-circuit si aucun favori n'a été trouvé
	if len(postIDs) == 0 {
		return []post_models.GetPostOutput{}
	}

	// 4. RÉSOLUTION ET HYDRATATION
	// On passe la liste d'IDs au pipeline unifié GetPosts qui va s'occuper :
	// - De vérifier les droits d'accès
	// - De filtrer les posts supprimés/privés
	// - De générer les URLs MinIO signées (HMAC)
	// - D'hydrater les commentaires
	return post_service.GetPosts(ctx, post_models.GetPostInput{
		UserID:  userID,
		PostIDs: postIDs,
	})
}
