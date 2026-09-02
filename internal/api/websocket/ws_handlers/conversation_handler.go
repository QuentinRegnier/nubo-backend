package ws_handlers

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
)

// HandleReadReceipt gère l'événement de lecture d'une conversation.
func HandleReadReceipt(ctx context.Context, callerID int64, rawPayload []byte) error {
	var input conversation_models.ReadReceiptInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nubo_error.NewBadRequest("INVALID_PAYLOAD", "Payload invalide.", err)
	}

	if err := pkg.ValidateStruct(&input); err != nil {
		return nubo_error.NewBadRequest("VALIDATION_FAILED", "Validation échouée.", err)
	}

	return conversation_service.MarkConversationAsRead(ctx, callerID, input.ConversationID)
}
