package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"go.mongodb.org/mongo-driver/bson"
)

// MongoGetMessageReactionsPaginated lit la liste des réactions d'un message depuis le L2
func MongoGetMessageReactionsPaginated(messageID int64, limit int64, offset int64) ([]message_models.MessageReactionPayload, error) {
	filter := bson.M{"message_id": messageID}
	sortMap := map[string]any{"created_at": -1}

	docs, err := MessageReactions.GetPaginated(filter, sortMap, offset, limit)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}

	var reactions []message_models.MessageReactionPayload
	for _, doc := range docs {
		var r message_models.MessageReactionPayload
		if err := pkg.ToStruct(doc, &r); err == nil {
			reactions = append(reactions, r)
		}
	}
	return reactions, nil
}
