package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

func MongoUpsertSaved(saved saved_models.SavedPayload) error {
	doc, err := pkg.ToMap(saved)
	if err != nil || doc == nil {
		return fmt.Errorf("erreur conversion saved pour Mongo")
	}
	return Saved.Set(doc)
}
