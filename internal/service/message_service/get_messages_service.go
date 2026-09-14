package message_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

func GetMessages(ctx context.Context, callerID int64, input message_models.GetMessagesInput) ([]message_models.MessageView, error) {
	// 1. SÉCURITÉ : Vérification d'appartenance
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return nil, nubo_error.NewForbidden("NOT_A_MEMBER", "Vous ne faites pas partie de cette conversation.", err)
	}

	// === NOUVEAU : CHARGEMENT DES SETTINGS DE LA CONVERSATION (Cascade L1 -> L2 -> L3) ===
	conv, errConv := object_cache_service.GetConversationFromObjectCache(ctx, input.ConversationID)
	if errConv != nil || conv.ID == 0 {
		conv, errConv = mongo.MongoGetConversation(input.ConversationID)
		if errConv != nil || conv.ID == 0 {
			// FALLBACK ABSOLU L3
			conv, errConv = postgres.FuncGetConversation(ctx, input.ConversationID)
			if errConv != nil || conv.ID == 0 {
				return nil, nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation introuvable.", errConv)
			}

			// ⬆️ PROMOTION L3 -> L2 (Asynchrone via la queue pour protéger Mongo)
			go func(c conversation_models.ConversationPayload) {
				bgCtx := context.Background()
				_ = redis.EnqueueDB(bgCtx, c.ID, c.ID, redis.EntityConversation, redis.ActionUpdate, c, redis.TargetMongo)
			}(conv)
		}

		// ⬆️ PROMOTION L3/L2 -> L1 (Immédiat en RAM)
		_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
	}

	hideSystemMessages := conv.Settings.HideSystemMessages

	// 2. RÉSOLUTION D'INDEX
	messageIDs, err := cache_service.GetMessageIDsFromSpeedCache(ctx, input.ConversationID, input.OffsetID, input.Limit, input.Direction, mem.FrozenMessageID)
	if err != nil || len(messageIDs) == 0 {
		return []message_models.MessageView{}, nil
	}

	// 3. HYDRATATION MASSIVE (L1 -> L2 -> L3)
	messages, err := object_cache_service.GetMessagesView(ctx, messageIDs)
	if err != nil {
		return nil, err
	}

	// 4. === SIGNATURE DES MÉDIAS, HYDRATATION DE L'EXPÉDITEUR & RÉACTIONS ===
	var views []message_models.MessageView

	for i := range messages {
		// === NOUVEAU : LE VRAI FILTRE EST ICI ===
		// Si la conversation cache les messages systèmes (Type 8), on les ignore à l'affichage
		if hideSystemMessages && messages[i].MessageType == 8 {
			continue
		}

		// A. Hydratation de l'image rattachée
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
					if view, err := media_service.GenerateMediaViewCascade(ctx, mediaID, messages[i].SenderID, messages[i].ID, callerID); err == nil {
						messages[i].Attachments["media_view"] = view
					}
				}
			}
		}

		// B. Hydratation du Pseudo et Avatar de l'expéditeur
		var senderUsername string
		var senderAvatar media_models.MediaView
		var senderAvatarCommunityID int64

		// Ajustement local
		if userLite, errLite := cache_service.GetUserLite(ctx, messages[i].SenderID); errLite == nil {
			senderUsername = userLite.Username
			if conv.Type == 2 || conv.Type == 3 {
				senderAvatarCommunityID = userLite.ProfilePictureID // Mode Twitch
			} else {
				if userLite.ProfilePictureID > 0 {
					if avatarView, errAvatar := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, messages[i].SenderID, 0, callerID); errAvatar == nil {
						senderAvatar = avatarView
					}
				}
			}
		}

		// C. HYDRATATION DES RÉACTIONS (FAST PATH)
		counts, _ := cache_service.GetMessageReactionCounts(ctx, messages[i].ID)
		userReaction, _ := cache_service.GetUserReaction(ctx, messages[i].ID, callerID)

		// D. Assemblage de la vue finale
		views = append(views, message_models.MessageView{
			MessagePayload:          messages[i],
			SenderUsername:          senderUsername,
			SenderAvatar:            senderAvatar,
			SenderAvatarCommunityID: senderAvatarCommunityID,
			ReactionCounts:          counts,
			UserReaction:            userReaction,
		})
	}

	if views == nil {
		views = make([]message_models.MessageView, 0)
	}

	return views, nil
}
