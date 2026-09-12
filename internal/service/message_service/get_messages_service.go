package message_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models" // ✅ NOUVEAU
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

func GetMessages(ctx context.Context, callerID int64, input message_models.GetMessagesInput) ([]message_models.MessageView, error) { // ✅ MODIFIÉ : Retourne des MessageView
	// 1. SÉCURITÉ : Vérification d'appartenance
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return nil, nubo_error.NewForbidden("NOT_A_MEMBER", "Vous ne faites pas partie de cette conversation.", err)
	}

	// 2. RÉSOLUTION D'INDEX
	messageIDs, err := cache_service.GetMessageIDsFromSpeedCache(ctx, input.ConversationID, input.OffsetID, input.Limit, input.Direction, mem.FrozenMessageID)
	if err != nil || len(messageIDs) == 0 {
		return []message_models.MessageView{}, nil // ✅ MODIFIÉ
	}

	// 3. HYDRATATION MASSIVE (L1 -> L2 -> L3)
	messages, err := object_cache_service.GetMessagesView(ctx, messageIDs)
	if err != nil {
		return nil, err
	}

	// 4. === SIGNATURE DES MÉDIAS & HYDRATATION DE L'EXPÉDITEUR ===
	var views []message_models.MessageView // ✅ NOUVEAU : Tableau final

	for i := range messages {
		// A. Hydratation de l'image rattachée (si c'est un message Média)
		if messages[i].Attachments != nil {
			if rawMediaID, exists := messages[i].Attachments["media_id"]; exists {
				var mediaID int64
				switch v := rawMediaID.(type) {
				case float64:
					mediaID = int64(v)
				case int64:
					mediaID = v
				}

				if mediaID > 0 {
					// Appel unique au domaine Média (DDD)
					if view, err := media_service.GenerateMediaViewCascade(ctx, mediaID, messages[i].SenderID, messages[i].ID, callerID); err == nil {
						messages[i].Attachments["media_view"] = view
					}
				}
			}
		}

		// B. Hydratation du Pseudo et Avatar de l'expéditeur (Sender)
		var senderUsername string
		var senderAvatar media_models.MediaView

		if userLite, errLite := cache_service.GetUserLite(ctx, messages[i].SenderID); errLite == nil {
			senderUsername = userLite.Username

			if userLite.ProfilePictureID > 0 {
				// authorID = SenderID, targetID = 0 (pas de post spécifique)
				if avatarView, errAvatar := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, messages[i].SenderID, 0, callerID); errAvatar == nil {
					senderAvatar = avatarView
				}
			}
		}

		// C. Assemblage de la vue finale
		views = append(views, message_models.MessageView{
			MessagePayload: messages[i],
			SenderUsername: senderUsername,
			SenderAvatar:   senderAvatar,
		})
	}

	return views, nil // ✅ MODIFIÉ
}
