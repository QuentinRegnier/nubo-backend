package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoGetConversation récupère le payload complet depuis le Cold Storage L2
func MongoGetConversation(convID int64) (conversation_models.ConversationPayload, error) {
	if Conversations == nil {
		return conversation_models.ConversationPayload{}, fmt.Errorf("mongo collection non initialisée")
	}

	filter := map[string]any{"id": convID}
	docs, err := Conversations.Get(filter, nil)
	if err != nil || len(docs) == 0 {
		return conversation_models.ConversationPayload{}, fmt.Errorf("conversation introuvable dans mongo")
	}

	var conv conversation_models.ConversationPayload
	if err := pkg.ToStruct(docs[0], &conv); err != nil {
		return conversation_models.ConversationPayload{}, err
	}

	return conv, nil
}
