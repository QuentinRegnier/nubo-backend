package media_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// ############################################################################
// # SERVICE : GÉNÉRATION D'URL SIGNÉE POUR LE CLIENT
// ############################################################################

// GetSignedURLForClient vérifie les droits d'accès en temps réel (Zero-Trust)
// et génère l'URL HMAC pour le client mobile ou web.
func GetSignedURLForClient(ctx context.Context, readerID, targetMediaID, contextPostID, contextConvID int64) (media_models.SignMediaOutput, error) {

	// ── ÉTAPE 1 : RÉCUPÉRATION DU MÉDIA (CASCADE L1 -> L2 -> L3) ────────────
	mediaPayload, errCascade := GetMediaCascade(ctx, targetMediaID)
	if errCascade != nil || !mediaPayload.Visibility {
		return media_models.SignMediaOutput{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Média introuvable ou supprimé.", errCascade)
	}

	// ── ÉTAPE 2 : MATRICE DE SÉCURITÉ CONTEXTUELLE ──────────────────────────
	var validatedContextID int64

	if contextPostID > 0 {
		// A. Cas d'une image attachée à une Publication (Post)
		postPayload, errSecurity := security_service.LeftPost(ctx, contextPostID, readerID)
		if errSecurity != nil {
			return media_models.SignMediaOutput{}, errSecurity // Propage l'AppError (Forbidden ou NotFound)
		}

		// Anti-Usurpation : Vérifier que le média demandé appartient réellement à ce post validé
		if !pkg.Exists(postPayload.MediaIDs, targetMediaID) {
			return media_models.SignMediaOutput{}, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Ce média n'appartient pas à cette publication.", nil)
		}
		validatedContextID = contextPostID

	} else if contextConvID > 0 {
		// B. Cas d'une image échangée dans une Messagerie Privée (Conversation)
		memberPayload, errSecurity := security_service.LeftMember(ctx, contextConvID, readerID)
		if errSecurity != nil || memberPayload.Role < 0 {
			return media_models.SignMediaOutput{}, nubo_error.NewForbidden(nubo_error.CodeForbidden, "Vous n'avez pas accès à cette conversation.", errSecurity)
		}
		validatedContextID = contextConvID

	} else {
		// C. Cas d'un Avatar public (Ni Post, Ni Conversation)
		// Les avatars de profil sont publics, la vérification est allégée (contexte 0)
		validatedContextID = 0
	}

	// ── ÉTAPE 3 : GÉNÉRATION DU SCEAU CRYPTOGRAPHIQUE ───────────────────────
	signedURL := GenerateWatermarkedURL(mediaPayload.StoragePath, mediaPayload.OwnerID, validatedContextID, readerID)

	return media_models.SignMediaOutput{
		MediaID: targetMediaID,
		URL:     signedURL,
	}, nil
}
