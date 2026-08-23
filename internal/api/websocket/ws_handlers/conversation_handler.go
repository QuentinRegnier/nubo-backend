package ws_handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
)

// HandleReadReceipt gère l'événement de lecture d'une conversation.
func HandleReadReceipt(ctx context.Context, callerID int64, rawPayload []byte) error {
	var input conversation_models.ReadReceiptInput

	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return errors.New("payload invalide")
	}

	// Appel du service métier existant (Cascade L1 -> L2 -> L3 + Write-Behind)
	return conversation_service.MarkConversationAsRead(ctx, callerID, input.ConversationID)
}
