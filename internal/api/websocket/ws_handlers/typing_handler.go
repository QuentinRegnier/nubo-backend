package ws_handlers

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

type TypingPayload struct {
	ConversationID int64 `json:"conversation_id" binding:"required"` // J'ai ajouté le binding required ici pour que la validation fonctionne
}

func HandleTyping(ctx context.Context, callerID int64, rawPayload []byte, isTyping bool) error {
	var input TypingPayload
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nubo_error.NewBadRequest("INVALID_PAYLOAD", "Payload invalide.", err)
	}

	if err := pkg.ValidateStruct(&input); err != nil {
		return nubo_error.NewBadRequest("VALIDATION_FAILED", "Validation échouée.", err)
	}

	// 1. SÉCURITÉ ZERO-TRUST : L1 Speed Cache
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil || mem.Role < 0 {
		return nubo_error.NewForbidden("ACCESS_DENIED", "Accès refusé.", err)
	}

	// 2. CHOIX DE L'ÉVÉNEMENT
	eventType := "typing.stopped"
	if isTyping {
		eventType = "typing.started"
	}

	// 3. BROADCAST VOLATIL (Directement via Redis Pub/Sub, Zéro BDD)
	broadcastPayload := map[string]any{
		"conversation_id": input.ConversationID,
		"user_id":         callerID,
	}

	return realtime_service.BroadcastToConversation(ctx, input.ConversationID, eventType, broadcastPayload)
}
