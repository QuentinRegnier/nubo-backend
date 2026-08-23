package message_service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// DeleteMessage gère la rétractation d'un message (Soft Delete).
func DeleteMessage(ctx context.Context, callerID int64, input message_models.DeleteMessageInput) error {
	// 1. SÉCURITÉ : Récupération du message complet et vérification des droits (L1 -> L2 -> L3)
	msg, err := security_service.LeftMessage(ctx, input.MessageID, callerID)
	if err != nil {
		return err // Retournera "unauthorized" ou "not found"
	}

	// 2. MODIFICATION DE L'ÉTAT (Soft Delete)
	msg.Visibility = false
	msg.UpdatedAt = time.Now().UTC()

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
				_ = fmt.Errorf("Failed to broadcast message deletion: %v", err)
			}
		}()
	}

	return err
}
