package message_service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UnreactToMessage gère le retrait ciblé d'une réaction sur un message.
func UnreactToMessage(ctx context.Context, callerID int64, input message_models.UnreactMessageInput) error {
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

	// 3. MANIPULATION DU JSONB (Attachments)[cite: 39]
	if msg.Attachments == nil {
		return nil // Optimisation (Idempotence) : pas d'attachments, donc aucune réaction à retirer[cite: 39]
	}

	reactions := make(map[string]any)
	if r, ok := msg.Attachments["reactions"]; ok {
		switch v := r.(type) {
		case map[string]any:
			reactions = v
		case map[any]any:
			for key, val := range v {
				if strKey, okKey := key.(string); okKey {
					reactions[strKey] = val
				}
			}
		}
	} else {
		return nil // Optimisation : la clé "reactions" n'existe pas, on arrête là[cite: 39]
	}

	// 4. RETRAIT DE LA RÉACTION[cite: 39]
	userIDStr := strconv.FormatInt(callerID, 10)

	// Vérification de présence (Idempotence)[cite: 39]
	if _, exists := reactions[userIDStr]; !exists {
		return nil // L'utilisateur n'avait pas réagi, on renvoie une réussite silencieuse[cite: 39]
	}

	// 🔒 RÈGLE : "Je ne supprime QUE ma réaction"
	// La fonction native delete() ne supprime que l'entrée correspondant à l'ID de l'appelant.
	// Toutes les autres clés (les réactions des autres membres) restent parfaitement intactes dans la map.[cite: 39]
	delete(reactions, userIDStr)

	// Réinjection dans l'objet global[cite: 39]
	msg.Attachments["reactions"] = reactions
	msg.UpdatedAt = time.Now().UTC()

	// 5. SAUVEGARDE EN RAM (L1)[cite: 39]
	_ = object_cache_service.SetMessageInObjectCache(ctx, msg)

	// 6. ENVOI AUX WORKERS (Write-Behind)
	err = redis.EnqueueDB(ctx, msg.ID, msg.ConversationID, redis.EntityMessage, redis.ActionUpdate, msg, redis.TargetAll)

	// 7. ENVOI NOTIFICATION (Asynchrone)
	if err == nil {
		go func() {
			err := realtime_service.BroadcastToConversation(context.Background(), msg.ConversationID, "message.unreacted", msg)
			if err != nil {
				_ = fmt.Errorf("Failed to broadcast message unreaction: %v", err)
			}
		}()
	}

	return err
}
