package notification_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// hydrateNotificationView assemble l'output final de la notification en récupérant
// le pseudo et l'avatar de l'acteur depuis le Speed Cache L1.
func hydrateNotificationView(ctx context.Context, callerID int64, notif notification_models.NotificationPayload) notification_models.NotificationView {
	view := notification_models.NotificationView{
		ID:        notif.ID,
		Type:      notif.Type,
		TargetID:  notif.TargetID,
		IsRead:    notif.IsRead,
		CreatedAt: notif.CreatedAt,
		ActorID:   notif.ActorID,
	}

	// 1. Lecture ultra-rapide O(1) de l'empreinte de l'acteur dans le Speed Cache
	if actorLite, err := cache_service.GetUserLite(ctx, notif.ActorID); err == nil {
		view.ActorUsername = actorLite.Username

		// 2. Appel du domaine Média pour signer l'URL de l'avatar
		if actorLite.ProfilePictureID > 0 {
			// authorID = actorLite.ID, targetID = 0 (pas de post spécifique lié à la photo de profil)
			if mediaView, errMedia := media_service.GenerateMediaViewCascade(ctx, actorLite.ProfilePictureID, actorLite.ID, 0, callerID); errMedia == nil {
				view.ActorAvatar = mediaView
			}
		}
	}

	return view
}
