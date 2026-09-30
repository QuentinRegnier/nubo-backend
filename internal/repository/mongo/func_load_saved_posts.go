package mongo

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"go.mongodb.org/mongo-driver/bson"
)

func MongoLoadSavedPosts(ctx context.Context, userID int64, limit int64, offset int64) ([]saved_models.SavedPayload, error) {
	filter := bson.M{"user_id": userID}
	sort := map[string]any{"created_at": -1}

	docs, err := Saved.GetPaginated(filter, sort, offset, limit)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
		return nil, nubo_error.NewInternal()
	}

	var saveds []saved_models.SavedPayload
	for _, doc := range docs {
		var s saved_models.SavedPayload
		if err := pkg.ToStruct(doc, &s); err == nil {
			saveds = append(saveds, s)
		}
	}
	return saveds, nil
}
