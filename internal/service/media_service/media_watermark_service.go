package media_service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
)

// GenerateWatermarkedURL crée une URL signée unique pour un seul média.
func GenerateWatermarkedURL(mediaKey string, authorID, postID, readerID int64) string {
	baseURL := os.Getenv("WATERMARK_API_URL")
	secret := os.Getenv("WATERMARK_SECRET_KEY")
	timestamp := time.Now().Unix()

	payload := fmt.Sprintf("key=%s&author=%d&post=%d&reader=%d&ts=%d", mediaKey, authorID, postID, readerID, timestamp)
	sig := security.GenerateHMAC(payload, secret)

	return fmt.Sprintf("%s/process?%s&sig=%s", baseURL, payload, sig)
}

// GenerateMediaViewCascade récupère le média (L1->L2->L3) et génère directement la vue signée (O(1) pour l'appelant).
func GenerateMediaViewCascade(ctx context.Context, mediaID, authorID, targetID, readerID int64) (media_models.MediaView, error) {
	mediaPayload, err := GetMediaCascade(ctx, mediaID)
	if err != nil || !mediaPayload.Visibility {
		return media_models.MediaView{}, errors.New("media introuvable ou supprimé")
	}

	signedURL := GenerateWatermarkedURL(mediaPayload.StoragePath, authorID, targetID, readerID)

	return media_models.MediaView{
		MediaID: mediaID,
		URL:     signedURL,
	}, nil
}

// FormatMediaViewsCascade prend une liste d'IDs, les hydrate, et retourne les MediaViews prêtes pour l'API.
func FormatMediaViewsCascade(ctx context.Context, mediaIDs []int64, authorID, targetID, readerID int64) []media_models.MediaView {
	var views []media_models.MediaView
	for _, id := range mediaIDs {
		// On ignore silencieusement les médias supprimés/invalides pour ne pas crasher tout le post
		if view, err := GenerateMediaViewCascade(ctx, id, authorID, targetID, readerID); err == nil {
			views = append(views, view)
		}
	}

	// Évite le retour 'null' en JSON
	if views == nil {
		views = make([]media_models.MediaView, 0)
	}
	return views
}
