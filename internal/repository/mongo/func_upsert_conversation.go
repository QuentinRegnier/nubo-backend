package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoUpsertConversation insère ou met à jour une conversation complète dans le Cold Storage L2.
func MongoUpsertConversation(conv conversation_models.ConversationPayload) error {
	doc, err := pkg.ToMap(conv)
	if err != nil || doc == nil {
		return fmt.Errorf("erreur conversion conversation_payload pour Mongo")
	}
	return Conversations.Set(doc)
}
