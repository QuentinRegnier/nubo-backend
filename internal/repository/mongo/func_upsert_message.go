package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoUpsertMessage répare le cache à froid
func MongoUpsertMessage(msg message_models.MessagePayload) error {
	doc, err := pkg.ToMap(msg)
	if err != nil || doc == nil {
		return nubo_error.NewInternal(err)
	}
	return Messages.Set(doc)
}
