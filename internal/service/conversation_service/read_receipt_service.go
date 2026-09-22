package conversation_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// MarkConversationAsRead remet à zéro le compteur de non-lus et émet l'événement WS si autorisé.
func MarkConversationAsRead(ctx context.Context, callerID int64, convID int64) (conversation_models.ReadReceiptOutput, error) {
	mem, err := security_service.LeftMember(ctx, convID, callerID)
	if err != nil {
		return conversation_models.ReadReceiptOutput{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Vous n'êtes pas membre de cette conversation.", err)
	}

	if mem.UnreadCount > 0 {
		// 1. MODIFICATION DE L'ÉTAT
		mem.UnreadCount = 0
		mem.UpdatedAt = domain.NowMillis()

		// 2. MISE À JOUR IMMÉDIATE L1 (Object Cache & Speed Cache)
		_ = object_cache_service.SetMemberInObjectCache(ctx, mem)
		_ = cache_service.ResetMemberUnreadCountInSpeedCache(ctx, convID, callerID)

		// 3. PERSISTANCE ASYNCHRONE (Write-Behind)
		_ = redis.EnqueueDB(ctx, mem.ID, convID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)
	}

	// 4. RÉCUPÉRATION DES PARAMÈTRES ET BLOCAGE CONDITIONNEL DU BROADCAST WS
	settings, errSet := object_cache_service.GetUserSettingsCascade(ctx, callerID)

	// ✅ APPLICATION: Send Read Receipts
	if errSet == nil && settings.Privacy.SendReadReceipts {
		payload := map[string]any{
			"conversation_id": convID,
			"user_id":         callerID,
		}
		// On broadcast la confirmation aux autres participants
		_ = realtime_service.BroadcastToConversation(ctx, convID, "conversation.read", payload)
	}

	// 5. DIRTY FLAG
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	return conversation_models.ReadReceiptOutput{
		InboxUpdateAt: domain.TimeToMillis(time.UnixMilli(timestampMs)),
	}, nil
}
