package ws_handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

type TypingPayload struct {
	ConversationID int64 `json:"conversation_id"`
}

// HandleTyping gère les événements typing.started et typing.stopped
func HandleTyping(ctx context.Context, callerID int64, rawPayload []byte, isTyping bool) error {
	var input TypingPayload
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return errors.New("payload invalide")
	}

	// 1. SÉCURITÉ ZERO-TRUST : L1 Speed Cache
	mem, err := security_service.LeftMember(ctx, input.ConversationID, callerID)
	if err != nil || mem.Role < 0 {
		return errors.New("accès refusé")
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
