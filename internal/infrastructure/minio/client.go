package minio

import (
	"context"
	"log"
	"os"

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
		log.Fatal("❌ Erreur critique : Impossible d'initialiser le client MinIO :", err)
	}

	// 3. Test de connexion (Ping)
	// On essaie une opération simple pour s'assurer que le serveur répond vraiment
	exists, err := MinioClient.BucketExists(context.Background(), bucketName)
	if err != nil {
		log.Printf("Attention : Connexion MinIO établie, mais impossible de vérifier le bucket '%s'. Erreur : %v", bucketName, err)
		// On ne fait pas forcément un Fatal ici, car le container 'createbuckets' peut prendre quelques secondes à démarrer
	} else if !exists {
		log.Printf("Attention : Le bucket '%s' n'existe pas encore.", bucketName)
	} else {
		log.Println("Connexion MinIO réussie et bucket vérifié.")
	}
}
