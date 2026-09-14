package message_service

import (
	"context"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
	"github.com/QuentinRegnier/nubo-backend/internal/worker"
)

// UnreactToMessage gère le retrait ciblé d'une réaction sur un message via le Hash Cache.
func UnreactToMessage(ctx context.Context, callerID int64, input message_models.UnreactMessageInput) error {
	// 1. SÉCURITÉ
	msg, err := security_service.LeftMessage(ctx, input.MessageID, callerID)
	if err != nil {
		return err
	}
	mem, err := security_service.LeftMember(ctx, msg.ConversationID, callerID)
	if err != nil || mem.Role < 0 {
		return nubo_error.NewForbidden("ACCESS_DENIED", "Accès refusé : vous ne faites pas partie de cette conversation.", err)
	}

	// 2. VÉRIFICATION DE PRÉSENCE (Idempotence en RAM L1)
	oldEmoji, _ := cache_service.GetUserReaction(ctx, msg.ID, callerID)
	if oldEmoji == "" {
		return nil // L'utilisateur n'avait pas réagi
	}

	// 3. MISE À JOUR DES COMPTEURS (RAM L1 & WORKER BATCH)
	_ = cache_service.IncrementReactionCount(ctx, msg.ID, oldEmoji, -1)
	worker.RegisterMessageReaction(msg.ID, oldEmoji, -1) // ✅ NOUVEAU

	_ = cache_service.DeleteUserReaction(ctx, msg.ID, callerID)

	// 4. PERSISTANCE ASYNCHRONE (Write-Behind)
	payload := message_models.MessageReactionPayload{
		MessageID: msg.ID,
		UserID:    callerID,
		// Les autres champs sont inutiles pour le DELETE
	}

	err = redis.EnqueueDB(ctx, msg.ID, msg.ConversationID, redis.EntityMessageReaction, redis.ActionDelete, payload, redis.TargetAll)

	// 5. DIFFUSION TEMPS RÉEL (WebSockets)
	if err == nil {
		go func() {
			bgCtx := context.Background()
			counts, _ := cache_service.GetMessageReactionCounts(bgCtx, msg.ID)

			msgView := message_models.MessageView{
				MessagePayload: msg,
				ReactionCounts: counts,
			}

			errWs := realtime_service.BroadcastToConversation(bgCtx, msg.ConversationID, "message.unreacted", msgView)
			if errWs != nil {
				_ = fmt.Errorf("Failed to broadcast message unreaction: %v", errWs)
			}
		}()
	}

	return err
}
