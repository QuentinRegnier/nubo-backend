package message_service

import (
	"context"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UpdateMessage gère la modification d'un message texte existant.
func UpdateMessage(ctx context.Context, callerID int64, input message_models.UpdateMessageInput) (message_models.UpdateMessageOutput, error) {
	// 1. SÉCURITÉ : Récupération du message complet et vérification d'appartenance
	msg, err := security_service.LeftMessage(ctx, input.MessageID, callerID)
	if err != nil {
		return message_models.UpdateMessageOutput{}, err // L'erreur est déjà formatée par LeftMessage
	}

	// 2. RÈGLE MÉTIER STRICTE : Seul le type 0 (Texte) est modifiable
	if msg.MessageType != 0 {
		return message_models.UpdateMessageOutput{}, nubo_error.NewBadRequest("UNSUPPORTED_MESSAGE_TYPE", "Seul un message de type texte peut être modifié.", nil)
	}

	// 3. APPLICATION DES MODIFICATIONS
	msg.Content = pkg.CleanStr(input.Content)
	if msg.Content == "" {
		return message_models.UpdateMessageOutput{}, nubo_error.NewBadRequest("EMPTY_MESSAGE", "Le message ne peut pas être vide.", nil)
	}
	msg.UpdatedAt = domain.NowMillis()

	// 4. MISE À JOUR IMMÉDIATE L1 (Object Cache)
	_ = object_cache_service.SetMessageInObjectCache(ctx, msg)

	// 5. ENVOI AUX WORKERS (Write-Behind)
	// PartitionKey = msg.ConversationID pour garantir l'ordre chronologique des opérations sur cette conversation
	err = redis.EnqueueDB(ctx, msg.ID, msg.ConversationID, redis.EntityMessage, redis.ActionUpdate, msg, redis.TargetAll)

	// 6. ENVOI NOTIFICATION (Asynchrone)
	if err == nil {
		go func() {
			bgCtx := context.Background()
			errWS := realtime_service.BroadcastToConversation(bgCtx, msg.ConversationID, "message.updated", msg)
			if errWS != nil {
				logger.Log.Error().Err(errWS).Msg("Failed to broadcast message update")
			}

			// ✅ NOUVEAU : SYNC LEDGER (Trigger granulaire)
			participantsStr, _ := redis.ConvParticipants.SMembers(bgCtx, msg.ConversationID)
			var pIDs []int64
			for _, p := range participantsStr {
				if id, err := strconv.ParseInt(p, 10, 64); err == nil {
					pIDs = append(pIDs, id)
				}
			}
			_ = cache_service.RecordMessageMutation(bgCtx, msg.ConversationID, msg.ID, pIDs)
		}()
	}

	output := message_models.UpdateMessageOutput{}

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
