package media_service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/minio"
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
		return fmt.Errorf("fichier invalide: %v", err)
	}
	if config.Width*config.Height > MaxPixels {
		return fmt.Errorf("image trop grande")
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("erreur lors de la réinitialisation du flux: %v", err)
	}

	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("erreur decode: %v", err)
	}

	if bounds := img.Bounds(); bounds.Dx() > MaxWidth {
		img = imaging.Resize(img, MaxWidth, 0, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := avif.Encode(&buf, img, avif.Options{Quality: 65, Speed: 5}); err != nil {
		return fmt.Errorf("erreur encodage avif: %v", err)
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
		return fmt.Errorf("erreur lors de l'envoi vers le stockage S3: %v", err)
	}

	// --- 3. CRÉATION DE L'OBJET ORPHELIN ---
	now := time.Now().UTC()
	media := models.MediaRequest{
		ID:          mediaID,
		OwnerID:     ownerID,
		StoragePath: storagePath,
		Visibility:  isVisible, // <-- S'adapte au contexte (Orphelin ou Direct)
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	ctx := context.Background()

	// --- 4. CACHE REDIS (Immédiat) ---
	if err := object_cache_service.SetMediaInObjectCache(ctx, media); err != nil {
		fmt.Printf("⚠️ Erreur Redis Media Set: %v\n", err)
	}

	// --- 5. PERSISTANCE ASYNCHRONE (Mongo + Postgres) ---
	err = redis.EnqueueDB(ctx, mediaID, ownerID, redis.EntityMedia, redis.ActionCreate, media, redis.TargetAll)

	if err != nil {
		log.Printf("🔥 CRITICAL: Impossible d'enqueue le Media %d : %v", mediaID, err)
		_ = minio.MinioClient.RemoveObject(context.Background(), bucketName, storagePath, miniogo.RemoveObjectOptions{})
		return fmt.Errorf("erreur systeme persistance: %v", err)
	}

	log.Printf("✅ Media %d uploadé (Visible: %t, Owner: %d)", mediaID, isVisible, ownerID)
	return nil
}
