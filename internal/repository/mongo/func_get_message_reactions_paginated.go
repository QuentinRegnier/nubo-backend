package mongo

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"go.mongodb.org/mongo-driver/bson"
)

// MongoGetMessageReactionsPaginated lit la liste des réactions d'un message depuis le L2
func MongoGetMessageReactionsPaginated(ctx context.Context, messageID int64, limit int64, offset int64) ([]message_models.MessageReactionPayload, error) {
	filter := bson.M{"message_id": messageID}
	sortMap := map[string]any{"created_at": -1}

	docs, err := MessageReactions.GetPaginated(filter, sortMap, offset, limit)
	if err != nil {
		numan_log.Error(ctx).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
		return nil, numan_error.NewInternal()
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
