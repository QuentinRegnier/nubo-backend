package media_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetMediaCascade récupère les informations d'un média L1 -> L2 -> L3 et réhydrate les caches
func GetMediaCascade(ctx context.Context, mediaID int64) (models.MediaRequest, error) {
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
		// ✅ HYDRATATION L2 (Mongo) ! La pièce manquante
		_ = mongo.MongoUpsertMedia(pgMedia)

		// ✅ HYDRATATION L1 (Redis)
		_ = object_cache_service.SetMediaInObjectCache(ctx, pgMedia)
		return pgMedia, nil
	}

	return models.MediaRequest{}, errors.New("media not found")
}
