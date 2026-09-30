package mongo

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoGetConversation récupère le payload complet depuis le Cold Storage L2
func MongoGetConversation(ctx context.Context, convID int64) (conversation_models.ConversationPayload, error) {
	if Conversations == nil {
		nubo_log.Error(ctx).Msg("La collection MongoDB 'Conversations' n'est pas initialisée")
		return conversation_models.ConversationPayload{}, nubo_error.NewInternal()
	}

	filter := map[string]any{"id": convID}
	docs, err := Conversations.Get(filter, nil)
	if err != nil || len(docs) == 0 {
		return conversation_models.ConversationPayload{}, nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation introuvable dans le stockage à froid.", err)
	}

	var conv conversation_models.ConversationPayload
	if err := pkg.ToStruct(docs[0], &conv); err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Échec de la conversion du document MongoDB en structure Go (ToStruct)")
		return conversation_models.ConversationPayload{}, nubo_error.NewInternal()
	}

	return conv, nil
}
