package media_service

import (
	"context"
	"os"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/minio"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	miniogo "github.com/minio/minio-go/v7"
)

// DeleteMedia orchestre la suppression complète d'un média sur toutes les couches (L1, L2, L3 et S3/L4).
func DeleteMedia(ctx context.Context, mediaID int64, ownerID int64) error {
	// 1. Récupération du chemin de stockage (Cascade L1 -> L2 -> L3)
	media, err := GetMediaCascade(ctx, mediaID)

	// 2. Destruction asynchrone sur le stockage S3 (Le L4) via la fonction dédiée
	if err == nil && media.StoragePath != "" {
		go func(path string) {
			_ = RemovePhysicalMedia(context.Background(), path)
		}(media.StoragePath)
	}

	// 3. Purge immédiate de la RAM (L1)
	_ = object_cache_service.DeleteMediaFromObjectCache(ctx, mediaID)

	// 4. Purge des bases de données (L2/L3) via la file d'attente asynchrone
	mediaPayload := models.MediaRequest{ID: mediaID, OwnerID: ownerID}
	return redis.EnqueueDB(ctx, mediaID, ownerID, redis.EntityMedia, redis.ActionDelete, mediaPayload, redis.TargetAll)
}

// RemovePhysicalMedia détruit physiquement le fichier sur MinIO (Respect du DDD).
// Utilisé par les suppressions unitaires et le Garbage Collector.
func RemovePhysicalMedia(ctx context.Context, storagePath string) error {
	if storagePath == "" {
		return nil
	}
	bucketName := os.Getenv("MINIO_BUCKET_NAME")
	if bucketName == "" {
		bucketName = "nubo-bucket"
	}
	return minio.MinioClient.RemoveObject(ctx, bucketName, storagePath, miniogo.RemoveObjectOptions{})
}
