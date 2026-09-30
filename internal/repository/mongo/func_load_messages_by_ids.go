package mongo

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoLoadMessagesByIDs est le fallback L2 d'hydratation
func MongoLoadMessagesByIDs(ctx context.Context, messageIDs []int64) ([]message_models.MessagePayload, error) {
	if len(messageIDs) == 0 || Messages == nil {
		return []message_models.MessagePayload{}, nil
	}

	filter := map[string]any{"id": map[string]any{"$in": messageIDs}}
	docs, err := Messages.Get(filter, nil)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
		return nil, nubo_error.NewInternal()
	}

	var messages []message_models.MessagePayload
	for _, doc := range docs {
		var m message_models.MessagePayload
		if err := pkg.ToStruct(doc, &m); err == nil {
			messages = append(messages, m)
		}
	}
	return messages, nil
}
