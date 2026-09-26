package notification_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// ############################################################################
// # UTILITAIRES : HYDRATATION ET ASSEMBLAGE DES VUES (DTO)
// ############################################################################

// hydrateNotificationView assemble l'output final de la notification en récupérant
// le pseudo et l'avatar signé cryptographiquement de l'acteur (Actor) via le Speed Cache L1.
func hydrateNotificationView(ctx context.Context, callerID int64, notificationPayload notification_models.NotificationPayload) notification_models.NotificationView {

	hydratedView := notification_models.NotificationView{
		ID:        notificationPayload.ID,
		Type:      notificationPayload.Type,
		TargetID:  notificationPayload.TargetID,
		IsRead:    notificationPayload.IsRead,
		CreatedAt: notificationPayload.CreatedAt,
		ActorID:   notificationPayload.ActorID,
	}

	// ── ÉTAPE 1 : LECTURE DE L'EMPREINTE LITE EN RAM L1 (O(1)) ──────────────

	if actorUserLite, errLite := cache_service.GetUserLite(ctx, notificationPayload.ActorID); errLite == nil {

		hydratedView.ActorUsername = actorUserLite.Username

		// ── ÉTAPE 2 : SIGNATURE HMAC DE L'AVATAR (DOMAINE MÉDIA) ────────────

		if actorUserLite.ProfilePictureID > 0 {
			// contextID = 0 (L'avatar n'est attaché ni à un Post ni à une Conversation)
			if generatedAvatarView, errMedia := media_service.GenerateMediaViewCascade(ctx, actorUserLite.ProfilePictureID, actorUserLite.ID, 0, callerID); errMedia == nil {
				hydratedView.ActorAvatar = generatedAvatarView
			}
		}
	}

	return hydratedView
}
