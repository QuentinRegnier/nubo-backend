package minio

import (
	"context"
	"os"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioClient est la variable globale qui sera utilisée par tes fonctions UploadMedia
var MinioClient *minio.Client

func InitMinio() {
	// 1. Récupération de la config depuis les variables d'environnement Docker
	endpoint := os.Getenv("MINIO_ENDPOINT")             // ex: "minio:9000"
	accessKeyID := os.Getenv("MINIO_ROOT_USER")         // ex: "nubo_minio_user"
	secretAccessKey := os.Getenv("MINIO_ROOT_PASSWORD") // ex: "nubo_minio_password..."

	bucketName := os.Getenv("MINIO_BUCKET_NAME")
	if bucketName == "" {
		bucketName = "nubo-bucket" // Fallback
	}

	// 2. Création du client
	var err error
	MinioClient, err = minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		// Secure doit être à FALSE car à l'intérieur du réseau Docker,
		// api1 parle à minio via HTTP (port 9000).
		// C'est Nginx (en frontal) qui gère le HTTPS.
		Secure: false,
	})
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Impossible d'initialiser le client MinIO")
	}

	// 3. Test de connexion (Ping)
	exists, err := MinioClient.BucketExists(context.Background(), bucketName)
	if err != nil {
		logger.Log.Warn().Err(err).Str("bucket", bucketName).Msg("Connexion MinIO établie, mais impossible de vérifier le bucket")
	} else if !exists {
		logger.Log.Warn().Str("bucket", bucketName).Msg("Le bucket n'existe pas encore.")
	} else {
		logger.Log.Info().Msg("Connexion MinIO réussie et bucket vérifié")
	}
}
