package ws_handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
)

func HandleCreateMessage(ctx context.Context, callerID int64, rawPayload []byte) (any, error) {
	var input message_models.CreateMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nil, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Payload invalide.", err)
	}
	if err := pkg.ValidateStruct(&input); err != nil {
		return nil, nubo_error.NewBadRequest("VALIDATION_FAILED", "Validation échouée.", err)
	}

	output, err := message_service.CreateMessage(ctx, callerID, input.ConversationID, input, false)
	if err != nil {
		return nil, err
	}

	// ICI on retourne la structure complète au lieu du map codé en dur !
	return output, nil
}

func HandleUpdateMessage(ctx context.Context, callerID int64, rawPayload []byte) (any, error) {
	var input message_models.UpdateMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nil, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Payload invalide.", err)
	}
	if err := pkg.ValidateStruct(&input); err != nil {
		return nil, errors.New("validation échouée : " + err.Error())
	}

	output, err := message_service.UpdateMessage(ctx, callerID, input)
	if err != nil {
		return nil, err
	}

	// ICI on retourne la structure complète au lieu du map codé en dur !
	return output, nil
}

func HandleDeleteMessage(ctx context.Context, callerID int64, rawPayload []byte) (any, error) {
	var input message_models.DeleteMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nil, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Payload invalide.", err)
	}
	if err := pkg.ValidateStruct(&input); err != nil {
		return nil, nubo_error.NewBadRequest("VALIDATION_FAILED", "Validation échouée.", err)
	}

	output, err := message_service.DeleteMessage(ctx, callerID, input)
	if err != nil {
		return nil, err
	}

	// ICI on retourne la structure complète au lieu du map codé en dur !
	return output, nil
}

func HandleReactMessage(ctx context.Context, callerID int64, rawPayload []byte) error {
	var input message_models.ReactMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nubo_error.NewBadRequest("INVALID_PAYLOAD", "Payload invalide.", err)
	}

	// 🛡️ BOUCLIER STATIQUE
	if err := pkg.ValidateStruct(&input); err != nil {
		return nubo_error.NewBadRequest("VALIDATION_FAILED", "Validation échouée.", err)
	}

	return message_service.ReactToMessage(ctx, callerID, input)
}

func HandleUnreactMessage(ctx context.Context, callerID int64, rawPayload []byte) error {
	var input message_models.UnreactMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nubo_error.NewBadRequest("INVALID_PAYLOAD", "Payload invalide.", err)
	}

	// 🛡️ BOUCLIER STATIQUE
	if err := pkg.ValidateStruct(&input); err != nil {
		return nubo_error.NewBadRequest("VALIDATION_FAILED", "Validation échouée.", err)
	}

	return message_service.UnreactToMessage(ctx, callerID, input)
}
