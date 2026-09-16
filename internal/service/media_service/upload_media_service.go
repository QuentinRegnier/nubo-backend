package media_service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/minio"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/disintegration/imaging"
	"github.com/gen2brain/avif"
	"github.com/google/uuid"
	miniogo "github.com/minio/minio-go/v7"
)

const (
	MaxPixels = 2000 * 2000
	MaxWidth  = 1920
)

// UploadMedia traite l'image (resize + AVIF), l'envoie sur MinIO et crée l'objet en BDD.
// Le paramètre "isVisible" permet de gérer la coexistence :
// - false : Upload Out-of-Band (Attente de confirmation WebSocket)
// - true  : Upload Direct (Création de Post classique)
func UploadMedia(file io.ReadSeeker, ownerID int64, mediaID int64, isVisible bool) error {
	// --- 1. ANALYSE & OPTIMISATION IMAGE (CPU Heavy) ---
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return nubo_error.NewBadRequest("INVALID_FILE", "Fichier image invalide ou corrompu.", err)
	}
	if config.Width*config.Height > MaxPixels {
		return nubo_error.NewBadRequest("IMAGE_TOO_LARGE", "L'image dépasse la résolution maximale autorisée.", nil)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nubo_error.NewInternal(err)
	}

	img, _, err := image.Decode(file)
	if err != nil {
		return nubo_error.NewBadRequest("DECODE_ERROR", "Erreur lors du décodage de l'image.", err)
	}

	if bounds := img.Bounds(); bounds.Dx() > MaxWidth {
		img = imaging.Resize(img, MaxWidth, 0, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := avif.Encode(&buf, img, avif.Options{Quality: 65, Speed: 5}); err != nil {
		return nubo_error.NewInternal(err)
	}

	// --- 2. UPLOAD VERS LE S3 (IO Network) ---
	objectName := fmt.Sprintf("%s.avif", uuid.New().String())
	storagePath := fmt.Sprintf("users/%d/media/%s", ownerID, objectName) // Rangement structuré

	bucketName := os.Getenv("MINIO_BUCKET_NAME")
	if bucketName == "" {
		bucketName = "nubo-bucket"
	}

	_, err = minio.MinioClient.PutObject(
		context.Background(),
		bucketName,
		storagePath,
		&buf,
		int64(buf.Len()),
		miniogo.PutObjectOptions{
			ContentType: "image/avif",
		},
	)
	if err != nil {
		return nubo_error.NewInternal(err) // Erreur infrastructure S3
	}

	// --- 3. CRÉATION DE L'OBJET ORPHELIN ---
	now := time.Now().UTC()
	media := media_models.MediaPayload{
		ID:          mediaID,
		OwnerID:     ownerID,
		StoragePath: storagePath,
		Visibility:  isVisible, // <-- S'adapte au contexte (Orphelin ou Direct)
		CreatedAt:   domain.TimeToMillis(now),
		UpdatedAt:   domain.TimeToMillis(now),
	}

	ctx := context.Background()

	// --- 4. CACHE REDIS (Immédiat) ---
	if err := object_cache_service.SetMediaInObjectCache(ctx, media); err != nil {
		logger.Log.Error().Err(err).Int64("media_id", mediaID).Msg("Erreur Redis Media Set")
	}

	// --- 5. PERSISTANCE ASYNCHRONE (Mongo + Postgres) ---
	err = redis.EnqueueDB(ctx, mediaID, ownerID, redis.EntityMedia, redis.ActionCreate, media, redis.TargetAll)

	if err != nil {
		logger.Log.Error().Err(err).Int64("media_id", mediaID).Msg("Impossible d'enqueue le Media")
		_ = minio.MinioClient.RemoveObject(context.Background(), bucketName, storagePath, miniogo.RemoveObjectOptions{})
		return nubo_error.NewInternal(err)
	}

	logger.Log.Info().
		Int64("media_id", mediaID).
		Bool("visible", isVisible).
		Int64("owner_id", ownerID).
		Msg("Media uploadé avec succès")
	return nil
}
