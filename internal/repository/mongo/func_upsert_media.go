package mongo

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoUpsertMedia met à jour ou insère un média dans le Cold Storage L2 lors d'une réhydratation depuis L3.
func MongoUpsertMedia(c context.Context, media media_models.MediaPayload) error {
	if Media == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"id": media.ID}
	update := bson.M{"$set": media}
	opts := options.Update().SetUpsert(true)

	_, err := Media.DB.Collection(Media.Name).UpdateOne(ctx, filter, update, opts)
	nubo_log.Error(c).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
	return nubo_error.NewInternal()
}
