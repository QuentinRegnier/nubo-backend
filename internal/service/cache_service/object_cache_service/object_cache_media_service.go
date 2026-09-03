package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// --- GESTION DES MÉDIAS (CACHE L1) ---

// SetMediaInObjectCache place les métadonnées de l'image en RAM (Write-Behind)
func SetMediaInObjectCache(ctx context.Context, media media_models.MediaPayload) error {
	// Le TTL (ex: 24h) est défini dans le manager Redis et se réinitialise à chaque GET
	return redis.Media.SetObject(ctx, media.ID, media)
}

// GetMediaFromObjectCache récupère instantanément les métadonnées (O(1))
func GetMediaFromObjectCache(ctx context.Context, mediaID int64) (media_models.MediaPayload, error) {
	var m media_models.MediaPayload
	err := redis.Media.GetObject(ctx, mediaID, &m)

	// ✅ Rejet immédiat si le média est soft-deleted
	if err == nil && !m.Visibility {
		return m, nubo_error.NewNotFound("MEDIA_DELETED", "Ce média a été supprimé.", nil)
	}

	return m, err
}

// DeleteMediaFromObjectCache purge les métadonnées de la RAM
func DeleteMediaFromObjectCache(ctx context.Context, mediaID int64) error {
	return redis.Media.DeleteObject(ctx, mediaID)
}
