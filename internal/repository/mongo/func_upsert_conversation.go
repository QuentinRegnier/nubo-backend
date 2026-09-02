package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoUpsertConversation insère ou met à jour une conversation complète dans le Cold Storage L2.
func MongoUpsertConversation(conv conversation_models.ConversationPayload) error {
	doc, err := pkg.ToMap(conv)
	if err != nil || doc == nil {
		return nubo_error.NewInternal(fmt.Errorf("erreur conversion payload pour Mongo: %w", err))
	}
	return Conversations.Set(doc)
}
