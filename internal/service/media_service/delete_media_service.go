package media_service

import (
	"context"
	"os"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/minio"
	miniogo "github.com/minio/minio-go/v7"
)

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
