package message_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

func GetMessages(ctx context.Context, callerID int64, input message_models.GetMessagesInput) ([]message_models.MessagePayload, error) {
	// 1. SÉCURITÉ : Vérification d'appartenance
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return nil, nubo_error.NewForbidden("NOT_A_MEMBER", "Vous ne faites pas partie de cette conversation.", err)
	}

	// 2. RÉSOLUTION D'INDEX
	messageIDs, err := cache_service.GetMessageIDsFromSpeedCache(ctx, input.ConversationID, input.OffsetID, input.Limit, input.Direction, mem.FrozenMessageID)
	if err != nil || len(messageIDs) == 0 {
		return []message_models.MessagePayload{}, nil
	}

	// 3. HYDRATATION MASSIVE (L1 -> L2 -> L3)
	messages, err := object_cache_service.GetMessagesView(ctx, messageIDs)
	if err != nil {
		return nil, err
	}

	// 4. === SIGNATURE DES MÉDIAS À LA VOLÉE ===
	for i := range messages {
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
						// FINI LE BRICOLAGE : On injecte l'objet MediaView entier par valeur
						messages[i].Attachments["media_view"] = view

						// Optionnel : On supprime l'ancien 'media_id' brut pour ne laisser que le MediaView propre
						// delete(messages[i].Attachments, "media_id")
					}
				}
			}
		}
	}

	return messages, nil
}
