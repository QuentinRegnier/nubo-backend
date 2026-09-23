package sync_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// SyncMessages récupère les payloads frais (et hydratés) des messages ayant muté depuis sinceMs.
func SyncMessages(ctx context.Context, callerID int64, input sync_models.SyncMessagesInput) (sync_models.SyncMessagesOutput, error) {
	// 1. SÉCURITÉ ZERO-TRUST : L'utilisateur doit être membre de la conversation
	_, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return sync_models.SyncMessagesOutput{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Vous n'êtes pas membre de cette conversation.", err)
	}

	// 2. RÉCUPÉRATION DES IDs DES MESSAGES AYANT MUTÉ (O(log N) L1 Cache)
	msgIDs, errCache := cache_service.GetModifiedMessageIDs(ctx, input.ConversationID, input.SinceMs)
	if errCache != nil {
		return sync_models.SyncMessagesOutput{}, nubo_error.NewInternal(errCache)
	}

	if len(msgIDs) == 0 {
		return sync_models.SyncMessagesOutput{Messages: []message_models.MessageView{}}, nil
	}

	// 3. HYDRATATION DES MESSAGES
	// On récupère le type de la conversation pour le formatage des avatars
	conv, _ := object_cache_service.GetConversationFromObjectCache(ctx, input.ConversationID)

	var views []message_models.MessageView
	var missingIDs []int64
	tempMsgs := make(map[int64]message_models.MessagePayload)

	// A. Lecture directe de l'Object Cache (L1)
	// On n'utilise pas object_cache_service.GetMessagesView car ce dernier filtre les messages avec "visibility = false".
	// Le client a BESOIN des messages supprimés pour mettre à jour son SQLite local.
	for _, msgID := range msgIDs {
		msg, errL1 := object_cache_service.GetMessageFromObjectCache(ctx, msgID)
		if errL1 == nil && msg.ID != 0 {
			tempMsgs[msg.ID] = msg
		} else {
			missingIDs = append(missingIDs, msgID)
		}
	}

	// B. Fallback L3 (Postgres) pour les Cache Misses éventuels
	if len(missingIDs) > 0 {
		pgMsgs, errPg := postgres.FuncLoadMessagesByIDs(ctx, missingIDs)
		if errPg == nil {
			for _, m := range pgMsgs {
				tempMsgs[m.ID] = m
				// Auto-guérison L1 silencieuse
				go func(payload message_models.MessagePayload) {
					_ = object_cache_service.SetMessageInObjectCache(context.Background(), payload)
				}(m)
			}
		}
	}

	// 4. ASSEMBLAGE ET HYDRATATION DES DTOs (Vues)
	for _, msgID := range msgIDs {
		msg, exists := tempMsgs[msgID]
		if !exists {
			continue
		}

		// Hydratation de l'image rattachée
		if msg.Attachments != nil {
			if rawMediaID, hasMedia := msg.Attachments["media_id"]; hasMedia {
				var mediaID int64
				switch v := rawMediaID.(type) {
				case float64:
					mediaID = int64(v)
				case int64:
					mediaID = v
				}
				if mediaID > 0 {
					if view, errMedia := media_service.GenerateMediaViewCascade(ctx, mediaID, msg.SenderID, msg.ID, callerID); errMedia == nil {
						msg.Attachments["media_view"] = view
					}
				}
			}
		}

		// Hydratation de l'expéditeur
		var senderUsername string
		var senderAvatar media_models.MediaView
		var senderAvatarCommunityID int64

		if userLite, errLite := cache_service.GetUserLite(ctx, msg.SenderID); errLite == nil {
			senderUsername = userLite.Username
			if conv.Type == 2 || conv.Type == 3 {
				senderAvatarCommunityID = userLite.ProfilePictureID // Twitch mode
			} else {
				if userLite.ProfilePictureID > 0 {
					if avatarView, errAvatar := media_service.GenerateMediaViewCascade(ctx, userLite.ProfilePictureID, msg.SenderID, 0, callerID); errAvatar == nil {
						senderAvatar = avatarView
					}
				}
			}
		}

		// Hydratation des réactions (Fast Path L1)
		counts, _ := cache_service.GetMessageReactionCounts(ctx, msg.ID)
		userReaction, _ := cache_service.GetUserReaction(ctx, msg.ID, callerID)

		views = append(views, message_models.MessageView{
			MessagePayload:          msg,
			SenderUsername:          senderUsername,
			SenderAvatar:            senderAvatar,
			SenderAvatarCommunityID: senderAvatarCommunityID,
			ReactionCounts:          counts,
			UserReaction:            userReaction,
		})
	}

	// Prévention du `null` JSON
	if views == nil {
		views = make([]message_models.MessageView, 0)
	}

	return sync_models.SyncMessagesOutput{
		Messages: views,
	}, nil
}
