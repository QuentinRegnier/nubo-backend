package message_service

import (
	"context"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// DeleteMessage gère la rétractation d'un message (Soft Delete).
func DeleteMessage(ctx context.Context, callerID int64, input message_models.DeleteMessageInput) (message_models.DeleteMessageOutput, error) {
	// 1. SÉCURITÉ : Récupération du message complet et vérification des droits (L1 -> L2 -> L3)
	msg, err := security_service.LeftMessage(ctx, input.MessageID, callerID)
	if err != nil {
		return message_models.DeleteMessageOutput{}, err // Retournera "unauthorized" ou "not found"
	}

	// 2. MODIFICATION DE L'ÉTAT (Soft Delete)
	msg.Visibility = false
	msg.UpdatedAt = domain.NowMillis()

	// 3. PURGE INSTANTANÉE DU CACHE L1 (RAM)
	// A. On détruit l'objet JSON pour libérer de la place
	_ = object_cache_service.DeleteMessageFromObjectCache(ctx, msg.ID)

	// B. On retire l'ID de l'Index ZSET de la conversation pour éviter de créer un "trou" à la prochaine requête GET
	_ = redis.MessagesIndex.ZRem(ctx, msg.ConversationID, strconv.FormatInt(msg.ID, 10))

	// 4. DÉLÉGATION DE LA PERSISTANCE AUX WORKERS (Write-Behind)
	// On envoie un ActionUpdate avec le payload complet modifié (Visibility = false).
	err = redis.EnqueueDB(ctx, msg.ID, msg.ConversationID, redis.EntityMessage, redis.ActionUpdate, msg, redis.TargetAll)

	// ENVOI NOTIFICATION (Asynchrone)
	if err == nil {
		go func() {
			err := realtime_service.BroadcastToConversation(context.Background(), msg.ConversationID, "message.deleted", msg)
			if err != nil {
				logger.Log.Error().Err(err).Msg("Failed to broadcast message deletion")
			}
		}()
	}

	output := message_models.DeleteMessageOutput{}

	// ========================================================================
	// MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	// Placé TOUT À LA FIN de la fonction. Cela écrase tout timestamp qui aurait
	// pu être généré précédemment (par ex. à l'intérieur de AddMembersToConversation)
	// et garantit que le client reçoit la date de la fin absolue de la transaction.
	timestampMs := cache_service.TouchInboxActivity(ctx, callerID)
	output.InboxUpdateAt = domain.TimeToMillis(time.UnixMilli(timestampMs))

	return output, err
}
