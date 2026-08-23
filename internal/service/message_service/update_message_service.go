package message_service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UpdateMessage gère la modification d'un message texte existant.
func UpdateMessage(ctx context.Context, callerID int64, input message_models.UpdateMessageInput) error {
	// 1. SÉCURITÉ : Récupération du message complet et vérification d'appartenance
	msg, err := security_service.LeftMessage(ctx, input.MessageID, callerID)
	if err != nil {
		return err
	}

	// 2. RÈGLE MÉTIER STRICTE : Seul le type 0 (Texte) est modifiable
	if msg.MessageType != 0 {
		return errors.New("unsupported type")
	}

	// 3. APPLICATION DES MODIFICATIONS
	msg.Content = pkg.CleanStr(input.Content)
	if msg.Content == "" {
		return errors.New("le message ne peut pas être vide")
	}
	msg.UpdatedAt = time.Now().UTC()

	// 4. MISE À JOUR IMMÉDIATE L1 (Object Cache)
	_ = object_cache_service.SetMessageInObjectCache(ctx, msg)

	// 5. ENVOI AUX WORKERS (Write-Behind)
	// PartitionKey = msg.ConversationID pour garantir l'ordre chronologique des opérations sur cette conversation
	err = redis.EnqueueDB(ctx, msg.ID, msg.ConversationID, redis.EntityMessage, redis.ActionUpdate, msg, redis.TargetAll)

	// 6. ENVOI NOTIFICATION (Asynchrone)
	if err == nil {
		go func() {
			err := realtime_service.BroadcastToConversation(context.Background(), msg.ConversationID, "message.updated", msg)
			if err != nil {
				_ = fmt.Errorf("Failed to broadcast message update: %v", err)
			}
		}()
	}

	return err
}
