package message_service

import (
	"context"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// ############################################################################
// # SERVICE : RÉTRACTATION D'UN MESSAGE (SOFT DELETE)
// ############################################################################

// DeleteMessage gère la suppression logique d'un message, retire son indexation L1,
// et propage l'événement de destruction aux clients connectés.
func DeleteMessage(ctx context.Context, callerID int64, input message_models.DeleteMessageInput) (message_models.DeleteMessageOutput, error) {

	// ── ÉTAPE 1 : CONTRÔLE D'ACCÈS ZERO-TRUST (CASCADE L1->L2->L3) ──────────

	messagePayload, errSecurity := security_service.LeftMessage(ctx, input.MessageID, callerID)
	if errSecurity != nil {
		return message_models.DeleteMessageOutput{}, errSecurity // Renvoie CodeNotFound ou CodeForbidden
	}

	// ── ÉTAPE 2 : APPLICATION DE LA SUPPRESSION (SOFT DELETE) ───────────────

	messagePayload.Visibility = false
	messagePayload.UpdatedAt = domain.NowMillis()

	// ── ÉTAPE 3 : PURGE INSTANTANÉE DU CACHE L1 (RAM) ───────────────────────

	// A. Destruction totale de l'objet JSON (Libération de la RAM pour les autres objets LFU)
	_ = object_cache_service.DeleteMessageFromObjectCache(ctx, messagePayload.ID)

	// B. Retrait de l'ID du ZSET de l'historique de la conversation pour éviter de créer un "trou" (nil) au prochain GET.
	_ = redis.MessagesIndex.ZRem(ctx, messagePayload.ConversationID, strconv.FormatInt(messagePayload.ID, 10))

	// ── ÉTAPE 4 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────

	// On demande une ActionUpdate avec le Visibility flag = false.
	errQueue := redis.EnqueueDB(ctx, messagePayload.ID, messagePayload.ConversationID, redis.EntityMessage, redis.ActionUpdate, messagePayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("msg_id", messagePayload.ID).Msg("Échec du Write-Behind lors de la suppression d'un message")
		return message_models.DeleteMessageOutput{}, nubo_error.NewInternal()
	}

	// ── ÉTAPE 5 : SYNCHRONISATION TEMPS RÉEL (ASYNCHRONE) ───────────────────

	go func() {
		backgroundCtx := context.Background()

		errBroadcast := realtime_service.BroadcastToConversation(backgroundCtx, messagePayload.ConversationID, "message.deleted", messagePayload)
		if errBroadcast != nil {
			logger.Log.Error().Err(errBroadcast).Msg("Échec de la diffusion WebSocket pour message.deleted")
		}

		// SYNC LEDGER (Trigger granulaire pour muter le SQLite des clients hors ligne)
		participantsStringList, _ := redis.ConvParticipants.SMembers(backgroundCtx, messagePayload.ConversationID)
		var syncTargetIDs []int64
		for _, participantStr := range participantsStringList {
			if parsedID, errParse := strconv.ParseInt(participantStr, 10, 64); errParse == nil {
				syncTargetIDs = append(syncTargetIDs, parsedID)
			}
		}
		_ = cache_service.RecordMessageMutation(backgroundCtx, messagePayload.ConversationID, messagePayload.ID, syncTargetIDs)
	}()

	// ── ÉTAPE 6 : MARQUAGE D'ACTIVITÉ (DIRTY FLAG) ──────────────────────────

	latestActivityTimestampMs := cache_service.TouchInboxActivity(ctx, callerID)

	return message_models.DeleteMessageOutput{
		InboxUpdateAt: domain.TimeToMillis(time.UnixMilli(latestActivityTimestampMs)),
	}, nil
}
