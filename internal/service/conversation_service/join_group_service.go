package conversation_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

func JoinGroup(ctx context.Context, callerID int64, input conversation_models.JoinGroupInput) error {
	// 1. AUTO-GUÉRISON (Cascade L1 -> L2 -> L3) POUR VÉRIFIER LA CONVERSATION
	conv, err := object_cache_service.GetConversationFromObjectCache(ctx, input.ConversationID)
	if err != nil || conv.ID == 0 {
		conv, err = mongo.MongoGetConversation(input.ConversationID)
		if err != nil || conv.ID == 0 {
			conv, err = postgres.FuncGetConversation(ctx, input.ConversationID)
			if err != nil || conv.ID == 0 {
				return nubo_error.NewNotFound("CONV_NOT_FOUND", "La conversation n'existe pas ou a été supprimée.", err)
			}
			_ = mongo.MongoUpsertConversation(conv)
		}
		_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
	}

	// 2. RÈGLE MÉTIER : On ne rejoint pas un MP.
	if conv.Type == 0 {
		return nubo_error.NewForbidden("INVALID_CONV_TYPE", "Impossible de rejoindre un message privé.", nil)
	}

	// 3. LA BARRIÈRE DE SÉCURITÉ
	if conv.Type == 1 || conv.Type == 2 {
		if input.InviteMsgID == 0 {
			return nubo_error.NewForbidden("INVITE_REQUIRED", "Une invitation est requise pour rejoindre ce groupe privé.", nil)
		}

		inviteMsg, errSec := security_service.LeftMessage(ctx, input.InviteMsgID, callerID)
		if errSec != nil {
			return nubo_error.NewNotFound("INVITE_NOT_FOUND", "Invitation introuvable ou vous n'en êtes pas le destinataire.", errSec)
		}

		if inviteMsg.MessageType != 6 {
			return nubo_error.NewBadRequest("INVALID_INVITE", "Le message fourni n'est pas une invitation valide.", nil)
		}

		if inviteMsg.Attachments == nil {
			return nubo_error.NewBadRequest("CORRUPT_INVITE", "Invitation corrompue (aucune cible).", nil)
		}

		targetConvRaw, exists := inviteMsg.Attachments["conversation_id"]
		if !exists {
			return nubo_error.NewBadRequest("MISSING_INVITE_TARGET", "Invitation invalide (cible manquante).", nil)
		}

		var targetConvID int64
		switch v := targetConvRaw.(type) {
		case float64:
			targetConvID = int64(v)
		case int64:
			targetConvID = v
		}

		if targetConvID != input.ConversationID {
			return nubo_error.NewForbidden("INVITE_MISMATCH", "Cette invitation ne correspond pas à ce groupe.", nil)
		}
	}
	// Si Type == 3, la porte est ouverte, on passe directement à la suite.

	// ========================================================================
	// 4. VÉRIFICATION DU STATUT DU MEMBRE
	// ========================================================================
	var mem conversation_models.MemberPayload
	var isUpdate bool

	mem, err = object_cache_service.GetMemberFromObjectCache(ctx, input.ConversationID, callerID)
	if err != nil || mem.ID == 0 {
		memPg, errPg := postgres.FuncGetMember(ctx, input.ConversationID, callerID)
		if errPg == nil && memPg.ID != 0 {
			mem = memPg
		}
	}

	if mem.ID != 0 {
		if mem.Role == -2 {
			return nubo_error.NewForbidden("USER_BANNED", "Vous êtes banni de ce groupe.", nil)
		}
		if mem.Role >= 0 {
			return nubo_error.NewBadRequest("ALREADY_MEMBER", "Vous faites déjà partie de ce groupe.", nil)
		}
		// Role = -1 : L'utilisateur revient après avoir quitté.
		isUpdate = true
	}

	// 5. CONSTRUCTION DE L'ÉTAT
	now := time.Now().UTC()
	if !isUpdate {
		mem = conversation_models.MemberPayload{
			ID:              pkg.GenerateID(),
			ConversationID:  input.ConversationID,
			UserID:          callerID,
			Role:            0, // Membre standard
			JoinedAt:        now,
			UnreadCount:     0,
			FrozenMessageID: 0,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	} else {
		mem.Role = 0
		mem.JoinedAt = now
		mem.UpdatedAt = now
		mem.FrozenMessageID = 0
	}

	// 6. CACHE L1 IMMÉDIAT
	_ = object_cache_service.SetMemberInObjectCache(ctx, mem)
	_ = cache_service.AddConversationToUserInbox(ctx, callerID, conv.ID, conv.LastMessageID)

	// === NOUVEAU : MISE À JOUR SYNCHRONE DU SPEED CACHE ===
	memLite := models.MemberLiteRequest{
		ConversationID:  mem.ConversationID,
		UserID:          mem.UserID,
		Role:            mem.Role,
		UnreadCount:     mem.UnreadCount,
		FrozenMessageID: mem.FrozenMessageID,
		JoinedAt:        mem.JoinedAt.UnixMilli(),
	}
	if isUpdate {
		_ = cache_service.UpdateMemberSpeedCache(ctx, memLite)
	} else {
		_ = cache_service.AddMemberToSpeedCache(ctx, memLite)
	}

	// 9. WRITE-BEHIND
	action := redis.ActionCreate
	if isUpdate {
		action = redis.ActionUpdate
	}
	err = redis.EnqueueDB(ctx, mem.ID, input.ConversationID, redis.EntityMembers, action, mem, redis.TargetAll)

	if err == nil {
		// 7. & 8. MESSAGE SYSTÈME ET NOTIFICATION (Asynchrone)
		go func() {
			bgCtx := context.Background()
			if callerLite, errLite := cache_service.GetUserLite(bgCtx, callerID); errLite == nil {
				sysContent := fmt.Sprintf("%s a rejoint le groupe", callerLite.Username)
				msgInput := message_models.CreateMessageInput{
					MessageType: 8,
					Content:     sysContent,
				}
				_, _ = message_service.CreateMessage(bgCtx, callerID, input.ConversationID, msgInput, true)
			}
			_ = realtime_service.BroadcastToConversation(bgCtx, input.ConversationID, "member.joined", mem)
		}()
	}

	return err
}
