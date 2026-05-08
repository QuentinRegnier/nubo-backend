package service

import (
	"fmt"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
)

// GenerateWatermarkedURL crée une URL signée unique pour un seul média.
func GenerateWatermarkedURL(mediaKey string, authorID, postID, readerID int64) string {
	baseURL := os.Getenv("WATERMARK_API_URL")
	secret := os.Getenv("WATERMARK_SECRET_KEY")
	timestamp := time.Now().Unix()

	// Construction de la chaîne à signer
	// L'ordre des paramètres doit être le même ici et dans le micro-service !
	payload := fmt.Sprintf("key=%s&author=%d&post=%d&reader=%d&ts=%d",
		mediaKey, authorID, postID, readerID, timestamp)

	// Génération de la signature via ton package security
	sig := security.GenerateHMAC(payload, secret)

	return fmt.Sprintf("%s/process?%s&sig=%s", baseURL, payload, sig)
}

// FormatMediaURLs est la fonction outil qui transforme une liste de médias en liste d'URLs signées.
func FormatMediaURLs(mediaList []domain.MediaRequest, authorID, postID, readerID int64) []string {
	urls := make([]string, len(mediaList))

	for i, m := range mediaList {
		// On utilise la clé de stockage (ex: "uuid.avif") pour générer l'URL
		urls[i] = GenerateWatermarkedURL(m.StoragePath, authorID, postID, readerID)
	}

	return urls
}
