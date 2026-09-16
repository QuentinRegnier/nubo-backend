package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

func MongoUpsertSaved(saved saved_models.SavedPayload) error {
	doc, err := pkg.ToMap(saved)
	if err != nil || doc == nil {
		return nubo_error.NewInternal(err)
	}
	return Saved.Set(doc)
}
