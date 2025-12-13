package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/cache" // <--- AJOUT IMPORT
	"github.com/QuentinRegnier/nubo-backend/internal/db"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

// ... (Les constantes et SetMinioClient restent inchangés) ...
const (
	MaxPixels = 2000 * 2000
	MaxWidth  = 1920
)

func SetMinioClient(client *minio.Client) { MinioClient = client }

func UploadMedia(file io.ReadSeeker, originalFilename string, ownerID string) (int, error) {

	// ... (Toute la partie 1 à 4 : Analyse, Encodage, Upload Minio reste identique) ...
	// --- 1. ANALYSE & 2. OPTIMISATION ---
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, fmt.Errorf("fichier invalide: %v", err)
	}
	if config.Width*config.Height > MaxPixels {
		return 0, fmt.Errorf("image trop grande")
	}
	file.Seek(0, 0)
	srcImg, err := imaging.Decode(file)
	if err != nil {
		return 0, fmt.Errorf("erreur decode: %v", err)
	}
	finalImg := imaging.Resize(srcImg, MaxWidth, 0, imaging.Lanczos)
	// --- 3. ENCODAGE ---
	var buf bytes.Buffer
	if err := webp.Encode(&buf, finalImg, &webp.Options{Lossless: false, Quality: 80}); err != nil {
		return 0, fmt.Errorf("erreur webp: %v", err)
	}
	// --- 4. UPLOAD MINIO ---
	bucketName := os.Getenv("MINIO_BUCKET_NAME")
	if bucketName == "" {
		bucketName = "nubo-bucket"
	}
	fileUUID := uuid.New().String()
	storagePath := fmt.Sprintf("%s/%s/%s.webp", fileUUID[0:2], fileUUID[2:4], fileUUID)
	_, err = MinioClient.PutObject(context.Background(), bucketName, storagePath, &buf, int64(buf.Len()), minio.PutObjectOptions{ContentType: "image/webp"})
	if err != nil {
		return 0, fmt.Errorf("erreur minio: %v", err)
	}

	// --- 5. SAUVEGARDE DB ---

	// A. Postgres (Logique existante)
	var dbOwnerID interface{}
	if ownerID == "" || ownerID == "system" {
		dbOwnerID = nil
	} else {
		dbOwnerID = ownerID
	}

	var newMediaID int
	err = db.PostgresDB.QueryRow(`
		SELECT content.func_create_media($1, $2)
	`, dbOwnerID, storagePath).Scan(&newMediaID)

	if err != nil {
		fmt.Printf("❌ ERREUR SQL FUNC_CREATE_MEDIA : %v\n", err)
		_ = MinioClient.RemoveObject(context.Background(), bucketName, storagePath, minio.RemoveObjectOptions{})
		return 0, fmt.Errorf("erreur sql: %v", err)
	}

	// B. Préparation Objet pour NoSQL (Mongo & Redis)

	// CORRECTION TYPE : MongoDB et Redis attendent un INT pour "owner_id".
	// Si on n'a pas d'ID (inscription), on met 0.
	var ownerIDInt int = 0
	// (Note: on ne tente pas de convertir "profile_xxx" en int, ça resterait 0, c'est ce qu'on veut)

	mediaObj := map[string]interface{}{
		"id":           newMediaID,
		"owner_id":     ownerIDInt, // <--- INT (0), pas String ("")
		"storage_path": storagePath,
		"created_at":   time.Now(),
		"visibility":   true,
	}

	// C. Mongo
	if err := db.Media.Set(mediaObj); err != nil {
		fmt.Printf("⚠️ Erreur Mongo Media Set: %v\n", err)
	}

	// D. Redis (AJOUT)
	if err := cache.RedisCreateMedia(mediaObj); err != nil {
		fmt.Printf("⚠️ Erreur Redis Media Set: %v\n", err)
	}

	return newMediaID, nil
}
