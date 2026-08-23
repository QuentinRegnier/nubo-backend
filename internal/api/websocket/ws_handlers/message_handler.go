package ws_handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/message_service"
)

func HandleCreateMessage(ctx context.Context, callerID int64, rawPayload []byte) (any, error) {
	var input message_models.CreateMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return nil, errors.New("payload invalide")
	}

	// Plus de paramètre "fileHeader" ! L'upload se fait Out-of-Band via HTTP.
	msgID, err := message_service.CreateMessage(ctx, callerID, input.ConversationID, input, false)
	if err != nil {
		return nil, err
	}

	// On retourne les données qui seront encapsulées dans la réponse WS "success"
	return map[string]int64{"message_id": msgID}, nil
}

func HandleUpdateMessage(ctx context.Context, callerID int64, rawPayload []byte) error {
	var input message_models.UpdateMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return errors.New("payload invalide")
	}
	return message_service.UpdateMessage(ctx, callerID, input)
}

func HandleDeleteMessage(ctx context.Context, callerID int64, rawPayload []byte) error {
	var input message_models.DeleteMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return errors.New("payload invalide")
	}
	return message_service.DeleteMessage(ctx, callerID, input)
}

func HandleReactMessage(ctx context.Context, callerID int64, rawPayload []byte) error {
	var input message_models.ReactMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return errors.New("payload invalide")
	}
	return message_service.ReactToMessage(ctx, callerID, input)
}

func HandleUnreactMessage(ctx context.Context, callerID int64, rawPayload []byte) error {
	var input message_models.UnreactMessageInput
	if err := json.Unmarshal(rawPayload, &input); err != nil {
		return errors.New("payload invalide")
	}
	return message_service.UnreactToMessage(ctx, callerID, input)
}
