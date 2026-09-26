package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : OBJECT CACHE (MÉDIAS LFU)
// ############################################################################

// SetMediaInObjectCache place les métadonnées de l'image en RAM (Write-Behind).
func SetMediaInObjectCache(ctx context.Context, mediaPayload media_models.MediaPayload) error {
	// Le TTL (ex: 24h) est défini dans le manager Redis et se réinitialise à chaque GET
	errRedis := redis.Media.SetObject(ctx, mediaPayload.ID, mediaPayload)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("media_id", mediaPayload.ID).Msg("Impossible de placer le média dans l'Object Cache")
		return nubo_error.NewInternal()
	}
	return nil
}

// GetMediaFromObjectCache récupère instantanément les métadonnées en O(1)
// avec un rejet immédiat si le média est soft-deleted.
func GetMediaFromObjectCache(ctx context.Context, mediaID int64) (media_models.MediaPayload, error) {
	var mediaPayload media_models.MediaPayload

	errRedis := redis.Media.GetObject(ctx, mediaID, &mediaPayload)
	if errRedis != nil {
		return media_models.MediaPayload{}, errRedis // Cache Miss standard
	}

	// Rejet immédiat si le média est soft-deleted
	if !mediaPayload.Visibility {
		return media_models.MediaPayload{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Ce média a été supprimé.", nil)
	}

	return mediaPayload, nil
}

// DeleteMediaFromObjectCache purge les métadonnées de la RAM L1.
func DeleteMediaFromObjectCache(ctx context.Context, mediaID int64) error {
	errRedis := redis.Media.DeleteObject(ctx, mediaID)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Int64("media_id", mediaID).Msg("Échec de la suppression du média de l'Object Cache")
		return nubo_error.NewInternal()
	}
	return nil
}
