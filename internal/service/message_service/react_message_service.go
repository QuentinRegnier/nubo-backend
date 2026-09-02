package message_service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// ReactToMessage gère l'ajout ou la modification d'une réaction sur un message.
func ReactToMessage(ctx context.Context, callerID int64, input message_models.ReactMessageInput) error {
	// 1. RÉCUPÉRATION DU MESSAGE (Auto-Guérison L1 -> L2 -> L3)
	messages, err := object_cache_service.GetMessagesView(ctx, []int64{input.MessageID})
	if err != nil || len(messages) == 0 {
		return nubo_error.NewNotFound("MESSAGE_NOT_FOUND", "Message introuvable ou supprimé.", err)
	}
	msg := messages[0]

	// 2. SÉCURITÉ : Vérification de l'appartenance à la conversation
	mem, err := security_service.LeftMember(ctx, msg.ConversationID, callerID)
	if err != nil || mem.Role < 0 {
		return nubo_error.NewForbidden("ACCESS_DENIED", "Accès refusé : vous ne faites pas partie de cette conversation.", err)
	}

	// 3. MANIPULATION DU JSONB (Attachments)[cite: 40]
	if msg.Attachments == nil {
		msg.Attachments = make(map[string]any)
	}

	// On isole le sous-dictionnaire des réactions[cite: 40]
	reactions := make(map[string]any)
	if r, ok := msg.Attachments["reactions"]; ok {
		switch v := r.(type) {
		case map[string]any:
			reactions = v
		case map[any]any: // Auto-correction si MsgPack décode les clés en type 'any'[cite: 40]
			for key, val := range v {
				if strKey, okKey := key.(string); okKey {
					reactions[strKey] = val
				}
			}
		}
	}

	// 4. AJOUT / MODIFICATION[cite: 40]
	// On convertit l'ID de l'appelant en chaîne de caractères pour s'en servir comme Clé[cite: 40]
	userIDStr := strconv.FormatInt(callerID, 10)
	input.Reaction = pkg.CleanStr(input.Reaction)

	// 🔒 RÈGLE : "1 utilisateur = 1 réaction maximum"
	// En utilisant l'ID de l'utilisateur comme clé du dictionnaire, s'il avait déjà une réaction,
	// elle est mathématiquement écrasée par la nouvelle. S'il n'en avait pas, elle est créée.[cite: 40]
	reactions[userIDStr] = input.Reaction

	// Réinjection dans l'objet global[cite: 40]
	msg.Attachments["reactions"] = reactions
	msg.UpdatedAt = time.Now().UTC()

	// 5. SAUVEGARDE EN RAM (L1)[cite: 40]
	_ = object_cache_service.SetMessageInObjectCache(ctx, msg)

	// 6. ENVOI AUX WORKERS (Write-Behind)
	// PartitionKey = msg.ConversationID pour garantir l'ordre chronologique en BDD
	err = redis.EnqueueDB(ctx, msg.ID, msg.ConversationID, redis.EntityMessage, redis.ActionUpdate, msg, redis.TargetAll)

	// 7. ENVOI NOTIFICATION (Asynchrone)
	if err == nil {
		go func() {
			err := realtime_service.BroadcastToConversation(context.Background(), msg.ConversationID, "message.reacted", msg)
			if err != nil {
				_ = fmt.Errorf("Failed to broadcast message reaction: %v", err)
			}
		}()
	}

	return err
}
