package member_service

import (
	"context"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// MuteMember gère l'interdiction de parler pour un membre.
func MuteMember(ctx context.Context, callerID int64, input member_models.MuteMemberInput) error {
	// 1. SÉCURITÉ : Vérifier que le Caller est Admin (1) ou Propriétaire (2)
	callerMem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil {
		return err
	}
	if callerMem.Role < 1 {
		return nubo_error.NewForbidden("NOT_ADMIN", "Seuls les administrateurs peuvent muter un membre.", nil)
	}

	// 2. SÉCURITÉ : Cible et hiérarchie
	targetMem, err := security_service.LeftMember(ctx, input.ConversationID, input.TargetUserID)
	if err != nil {
		return err
	}
	if targetMem.Role >= callerMem.Role {
		return nubo_error.NewForbidden("HIERARCHY_ERROR", "Vous ne pouvez pas muter un membre de rang égal ou supérieur.", nil)
	}

	// 3. LOGIQUE ALGORITHMIQUE : Ne pas muter quelqu'un qui n'a déjà pas le droit de parler
	conv, _ := object_cache_service.GetConversationFromObjectCache(ctx, input.ConversationID)
	if conv.ID == 0 {
		// TENTATIVE L2 (MongoDB)
		conv, _ = mongo.MongoGetConversation(input.ConversationID)
		if conv.ID != 0 {
			// =========================================================
			// RÉHYDRATATION L2 -> L1
			// =========================================================
			_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
		} else {
			// FALLBACK ABSOLU L3 (PostgreSQL)
			conv, _ = postgres.FuncGetConversation(ctx, input.ConversationID)
			if conv.ID != 0 {
				// =========================================================
				// RÉHYDRATATION L3 -> L2 & L1
				// =========================================================

				// PROMOTION L3 -> L2 (Asynchrone via la queue)
				go func(c conversation_models.ConversationPayload) {
					bgCtx := context.Background()
					_ = redis.EnqueueDB(bgCtx, c.ID, c.ID, redis.EntityConversation, redis.ActionUpdate, c, redis.TargetMongo)
				}(conv)

				// PROMOTION L3 -> L1 (Immédiat en RAM)
				_ = object_cache_service.SetConversationInObjectCache(ctx, conv)
			}
		}
	}

	if conv.ID != 0 {
		// Si la WritePermission est à 1 (Admins seulement) et que la cible est membre (0)
		if conv.Settings.WritePermission == 1 && targetMem.Role == 0 {
			return nubo_error.NewForbidden("ALREADY_MUTED_BY_LAWS", "Ce membre n'a déjà pas le droit de parole dans cette communauté à cause des permissions de base.", nil)
		}
	} else {
		return nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation introuvable ou inactive.", err)
	}

	// 4. APPLICATION DE LA PUNITION
	targetMem.Settings.RestrictedUntil = input.RestrictedUntil
	targetMem.UpdatedAt = domain.NowMillis()

	// 5. MISE À JOUR L1 INSTANTANÉE
	memberID := fmt.Sprintf("%d:%d", input.ConversationID, input.TargetUserID)
	var memLite lite_models.MemberLiteRequest
	if errCache := redis.ConvMembers.GetObject(ctx, memberID, &memLite); errCache == nil {
		memLite.Settings.RestrictedUntil = input.RestrictedUntil
		_ = redis.ConvMembers.SetObject(ctx, memberID, memLite)
	}

	// 6. PERSISTANCE ASYNCHRONE
	errQueue := redis.EnqueueDB(ctx, targetMem.ID, input.ConversationID, redis.EntityMembers, redis.ActionUpdate, targetMem, redis.TargetAll)

	if errQueue == nil {
		// 7. DIFFUSION TEMPS RÉEL DE L'ÉTAT DU MEMBRE
		go func() {
			bgCtx := context.Background()
			_ = realtime_service.BroadcastToConversation(bgCtx, input.ConversationID, "member.muted", targetMem)
		}()

		// 8. ENVOI DU MESSAGE SYSTÈME (Dans le flux de la conversation)
		sysMsgInput := message_models.CreateMessageInput{
			ConversationID: input.ConversationID,
			MessageType:    8, // 8 = System Event
			Content:        "Un membre a été restreint.",
			Attachments: map[string]any{
				"event_type":       "member_muted",
				"target_user_id":   input.TargetUserID,
				"restricted_until": input.RestrictedUntil,
			},
		}
		// On envoie en "isInternal = true" pour bypasser les droits de l'admin s'il est par hasard en mode lecture
		_, _ = message_service.CreateMessage(ctx, callerID, input.ConversationID, sysMsgInput, true)
	}

	return errQueue
}
