package mongo

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoUpsertMedia met à jour ou insère un média dans le Cold Storage L2 lors d'une réhydratation depuis L3.
func MongoUpsertMedia(media models.MediaRequest) error {
	if Media == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"id": media.ID}
	update := bson.M{"$set": media}
	opts := options.Update().SetUpsert(true)

	_, err := Media.DB.Collection(Media.Name).UpdateOne(ctx, filter, update, opts)
	return err
}
