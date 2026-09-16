package conversation_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// MarkConversationAsRead remet le compteur de messages non lus à 0 pour l'utilisateur.
func MarkConversationAsRead(ctx context.Context, callerID int64, convID int64) (conversation_models.ReadReceiptOutput, error) {
	mem, err := security_service.LeftMember(ctx, convID, callerID)
	if err != nil {
		return conversation_models.ReadReceiptOutput{}, err
	}

	// Même si le compteur est déjà à 0, on renvoie un timestamp valide pour le WS
	if mem.UnreadCount == 0 {
		return conversation_models.ReadReceiptOutput{
			InboxUpdateAt: time.Now().UnixMilli(),
		}, nil
	}

	mem.UnreadCount = 0
	mem.UpdatedAt = service.NowMillis()

	err = redis.EnqueueDB(ctx, mem.ID, convID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)
	if err != nil {
		return conversation_models.ReadReceiptOutput{}, err
	}

	_ = object_cache_service.SetMemberInObjectCache(ctx, mem)
	_ = cache_service.ResetMemberUnreadCountInSpeedCache(ctx, convID, callerID)

	go func() {
		err := realtime_service.BroadcastToConversation(context.Background(), convID, "conversation.read_receipt", mem)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de l'envoi de la notification de lecture")
		}
	}()

	// ========================================================================
	// MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)

	return conversation_models.ReadReceiptOutput{
		InboxUpdateAt: timestampMs, // Ton modèle attend bien un int64 ici
	}, nil
}
