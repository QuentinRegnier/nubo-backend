package media_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// GetSignedURLForClient vérifie les droits d'accès en temps réel et génère l'URL HMAC pour le client mobile.
func GetSignedURLForClient(ctx context.Context, readerID, mediaID, postID, convID int64) (media_models.SignMediaOutput, error) {
	// 1. Récupération du Média en O(1)
	mediaPayload, err := GetMediaCascade(ctx, mediaID)
	if err != nil || !mediaPayload.Visibility {
		return media_models.SignMediaOutput{}, nubo_error.NewNotFound("MEDIA_NOT_FOUND", "Média introuvable ou supprimé.", err)
	}

	// 2. Vérification des droits en temps réel (Zero-Trust)
	var contextID int64

	if postID > 0 {
		// A. Cas d'une image dans un Post
		post, errSec := security_service.LeftPost(ctx, postID, readerID)
		if errSec != nil {
			return media_models.SignMediaOutput{}, errSec // Propage l'erreur de LeftPost (Forbidden ou NotFound)
		}
		// S'assurer que le média appartient bien à ce post (pour éviter qu'on passe un post valide mais un media volé)
		if !pkg.Exists(post.MediaIDs, mediaID) {
			return media_models.SignMediaOutput{}, nubo_error.NewForbidden("INVALID_CONTEXT", "Ce média n'appartient pas à cette publication.", nil)
		}
		contextID = postID

	} else if convID > 0 {
		// B. Cas d'une image dans un Message (Conversation)
		mem, errSec := security_service.LeftMember(ctx, convID, readerID)
		if errSec != nil || mem.Role < 0 {
			return media_models.SignMediaOutput{}, nubo_error.NewForbidden("ACCESS_DENIED", "Vous n'avez pas/plus accès à cette conversation.", errSec)
		}
		contextID = convID

	} else {
		// C. Cas d'un Avatar (ni Post, ni Conversation)
		// Les avatars de profil sont publics, on laisse passer avec un contextID de 0.
		contextID = 0
	}

	// 3. Génération de l'URL signée HMAC
	signedURL := GenerateWatermarkedURL(mediaPayload.StoragePath, mediaPayload.OwnerID, contextID, readerID)

	return media_models.SignMediaOutput{
		MediaID: mediaID,
		URL:     signedURL,
	}, nil
}
