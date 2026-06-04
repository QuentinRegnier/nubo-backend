package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// --- GESTION DES MÉDIAS (CACHE L1) ---

// SetMediaInObjectCache place les métadonnées de l'image en RAM (Write-Behind)
func SetMediaInObjectCache(ctx context.Context, media models.MediaRequest) error {
	// Le TTL (ex: 24h) est défini dans le manager Redis et se réinitialise à chaque GET
	return redis.Media.SetObject(ctx, media.ID, media)
}

// GetMediaFromObjectCache récupère instantanément les métadonnées (O(1))
func GetMediaFromObjectCache(ctx context.Context, mediaID int64) (models.MediaRequest, error) {
	var m models.MediaRequest
	err := redis.Media.GetObject(ctx, mediaID, &m)
	return m, err
}

// DeleteMediaFromObjectCache purge les métadonnées de la RAM
func DeleteMediaFromObjectCache(ctx context.Context, mediaID int64) error {
	return redis.Media.DeleteObject(ctx, mediaID)
}
