package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/minio"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/google/uuid"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/avif"
	miniogo "github.com/minio/minio-go/v7"
)

// ... (Les constantes et SetMinioClient restent inchangés) ...
const (
	MaxPixels = 2000 * 2000
	MaxWidth  = 1920
)

func UploadMedia(file io.ReadSeeker, ownerID int64, mediaID int64) error {

	// --- 1. ANALYSE & OPTIMISATION IMAGE (CPU Heavy) ---
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return fmt.Errorf("fichier invalide: %v", err)
	}

	if config.Width*config.Height > MaxPixels {
		return fmt.Errorf("image trop grande")
	}

	// ✅ CORRECTION SEEK : Gestion de l'erreur
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
	// ✅ CORRECTION AVIF : Retrait du '&'
	if err := avif.Encode(&buf, img, avif.Options{Quality: 65, Speed: 5}); err != nil {
		return fmt.Errorf("erreur encodage avif: %v", err)
	}

	// --- 2. UPLOAD VERS LE S3 (IO Network) ---
	// Nous utilisons désormais l'extension .avif
	objectName := fmt.Sprintf("%s.avif", uuid.New().String())
	bucketName := os.Getenv("MINIO_BUCKET_NAME")
	storagePath := objectName

	// Configuration indispensable du ContentType en "image/avif" pour Scaleway/MinIO
	_, err = minio.MinioClient.PutObject(
		context.Background(),
		bucketName,
		objectName,
		&buf,
		int64(buf.Len()),
		miniogo.PutObjectOptions{
			ContentType: "image/avif",
			// Optionnel : On peut ajouter des métadonnées personnalisées ici si besoin
		},
	)
	if err != nil {
		return fmt.Errorf("erreur lors de l'envoi vers le stockage S3: %v", err)
	}

	// --- 3. CRÉATION DE L'OBJET & ID SNOWFLAKE (Go Authority) ---
	now := time.Now().UTC()

	media := domain.MediaRequest{
		ID:          mediaID,
		OwnerID:     ownerID,
		StoragePath: storagePath,
		Visibility:  true, // Par défaut visible, à ajuster selon ta logique
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// --- 5. CACHE REDIS (Immédiat) ---
	// On met en cache pour que l'UI puisse afficher l'image tout de suite si besoin
	// Clé ex: "media:12345"

	// On écrit directement dans Redis (Set avec expiration par exemple 24h)
	if err := redis.RedisCreateMedia(media); err != nil {
		fmt.Printf("⚠️ Erreur Redis Media Set: %v\n", err)
	}

	// --- 6. PERSISTANCE ASYNCHRONE (Mongo + Postgres) ---
	// C'est ici qu'on remplace les appels SQL directs par la file d'attente
	ctx := context.Background()

	err = redis.EnqueueDB(
		ctx,
		mediaID,
		ownerID,
		redis.EntityMedia, // Assure-toi d'avoir défini cette constante dans async_queue.go
		redis.ActionCreate,
		media,           // Le payload complet
		redis.TargetAll, // Mongo ET Postgres
	)

	if err != nil {
		// Cas critique : Si Redis échoue, on supprime l'image de Minio pour ne pas laisser de fichiers orphelins
		// (Ou on logge une erreur critique)
		log.Printf("❌ CRITICAL: Impossible d'enqueue le Media %d : %v", mediaID, err)
		_ = minio.MinioClient.RemoveObject(context.Background(), bucketName, objectName, miniogo.RemoveObjectOptions{})
		return fmt.Errorf("erreur systeme persistance: %v", err)
	}

	log.Printf("✅ Media %d uploadé et mis en file d'attente (Owner: %d)", mediaID, ownerID)
	return nil
}
