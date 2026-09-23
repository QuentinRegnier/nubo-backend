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

// MarkConversationAsRead remet à zéro le compteur de non-lus, met à jour le curseur de lecture (Watermark)
// et émet l'événement WS si autorisé et si le groupe n'est pas trop grand.
func MarkConversationAsRead(ctx context.Context, callerID int64, convID int64) (conversation_models.ReadReceiptOutput, error) {
	// 1. SÉCURITÉ : Vérification de l'appartenance à la conversation
	mem, err := security_service.LeftMember(ctx, convID, callerID)
	if err != nil {
		return conversation_models.ReadReceiptOutput{}, nubo_error.NewForbidden("NOT_A_MEMBER", "Vous n'êtes pas membre de cette conversation.", err)
	}

	// 2. RÉCUPÉRATION DU DERNIER MESSAGE DE LA CONVERSATION
	conv, errConv := object_cache_service.GetConversationFromObjectCache(ctx, convID)
	if errConv != nil || conv.ID == 0 {
		return conversation_models.ReadReceiptOutput{}, nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation introuvable.", errConv)
	}

	hasChanged := false

	// A. Remise à zéro des non-lus (UnreadCount)
	if mem.UnreadCount > 0 {
		mem.UnreadCount = 0
		hasChanged = true
	}

	// B. Mise à jour du Watermark (LastReadMessageID)
	// On ne recule jamais un curseur de lecture, on s'assure de toujours avancer.
	if conv.LastMessageID > mem.LastReadMessageID {
		mem.LastReadMessageID = conv.LastMessageID
		hasChanged = true

		// ✅ NOUVEAU : ENREGISTREMENT INSTANTANÉ DANS LE HASH REDIS L1 (O(1))
		// Cela permet à la future route GET /watermarks de lire la donnée sans saturer la BDD.
		_ = cache_service.SetWatermarkInSpeedCache(ctx, convID, callerID, conv.LastMessageID)
	}

	// 3. PERSISTANCE (Uniquement si quelque chose a vraiment changé)
	if hasChanged {
		mem.UpdatedAt = domain.NowMillis()

		// Mise à jour de l'Object Cache (Complet) et du Speed Cache (Lite)
		_ = object_cache_service.SetMemberInObjectCache(ctx, mem)
		// Note : ResetMemberUnreadCountInSpeedCache devra être mis à jour pour aussi rafraîchir LastReadMessageID
		_ = cache_service.ResetMemberUnreadCountInSpeedCache(ctx, convID, callerID, conv.LastMessageID)

		// Persistance asynchrone (Write-Behind)
		_ = redis.EnqueueDB(ctx, mem.ID, convID, redis.EntityMembers, redis.ActionUpdate, mem, redis.TargetAll)
	}

	// 4. RÉCUPÉRATION DES PARAMÈTRES ET LE VIDEUR WEBSOCKET
	settings, errSet := object_cache_service.GetUserSettingsCascade(ctx, callerID)

	// L'utilisateur a-t-il autorisé l'envoi d'accusés de réception ?
	if errSet == nil && settings.Privacy.SendReadReceipts {

		// ✅ LE VIDEUR (CAPPING) : Vérification de la taille du groupe en O(1)
		participantCount, _ := redis.ConvParticipants.SCard(ctx, convID)

		// Si le groupe dépasse 50 membres, on BLOQUE l'événement WebSocket
		// pour prévenir la tempête d'acquittements (ACK Storm).
		if participantCount <= 50 {
			payload := map[string]any{
				"conversation_id":      convID,
				"user_id":              callerID,
				"last_read_message_id": mem.LastReadMessageID, // Envoi du curseur pour l'animation UI
			}
			// Broadcast de la confirmation aux autres participants
			_ = realtime_service.BroadcastToConversation(ctx, convID, "conversation.read", payload)
		}
	}

	// 5. DIRTY FLAG
	// On marque toujours l'activité pour la synchronisation, même s'il n'y a pas eu de vrai changement,
	// car un simple "ReadReceipt" du client signifie qu'il est actif sur cette interface.
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	return conversation_models.ReadReceiptOutput{
		InboxUpdateAt: domain.TimeToMillis(time.UnixMilli(timestampMs)),
	}, nil
}
