package media_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetMediaCascade récupère les informations d'un média L1 -> L2 -> L3 et réhydrate les caches
func GetMediaCascade(ctx context.Context, mediaID int64) (media_models.MediaPayload, error) {
	// 1. Tente le L1 (RAM)
	if m, err := object_cache_service.GetMediaFromObjectCache(ctx, mediaID); err == nil {
		return m, nil
	}

	// 2. Tente le L2 (Mongo)
	mongoMedia, errMongo := mongo.MongoLoadMedia([]int64{mediaID})
	if errMongo == nil && len(mongoMedia) > 0 {
		_ = object_cache_service.SetMediaInObjectCache(ctx, mongoMedia[0])
		return mongoMedia[0], nil
	}

	// 3. Fallback L3 (Postgres)
	if pgMedia, errPg := postgres.FuncGetMedia(ctx, mediaID); errPg == nil {
		// ✅ HYDRATATION L1 (Redis) Synchrone
		_ = object_cache_service.SetMediaInObjectCache(ctx, pgMedia)

		// ✅ HYDRATATION L2 (Mongo) Asynchrone
		go func(m media_models.MediaPayload) {
			bgCtx := context.Background()
			// PartitionKey = OwnerID
			_ = redis.EnqueueDB(bgCtx, m.ID, m.OwnerID, redis.EntityMedia, redis.ActionUpdate, m, redis.TargetMongo)
		}(pgMedia)

		return pgMedia, nil
	}

	return media_models.MediaPayload{}, nubo_error.NewNotFound("MEDIA_NOT_FOUND", "Média introuvable.", nil)
}
