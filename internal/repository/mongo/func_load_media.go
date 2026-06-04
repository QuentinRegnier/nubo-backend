package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models" // ✅ Le bon import
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func MongoLoadMedia(mediaIDs []int64) ([]models.MediaRequest, error) {
	if len(mediaIDs) == 0 || Media == nil {
		return []models.MediaRequest{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"id": bson.M{"$in": mediaIDs}}

	cursor, err := Media.DB.Collection(Media.Name).Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			fmt.Println(err)
		}
	}(cursor, ctx)

	var results []models.MediaRequest
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return results, nil
}
