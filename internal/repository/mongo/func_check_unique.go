package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"go.mongodb.org/mongo-driver/bson"
)

func MongoCheckUnique(entity redis.EntityType, field string, value any) (bool, error) {
	collection, err := Redis2Mongo(entity)
	if err != nil {
		return false, nubo_error.NewInternal(err) // Coupe-circuit immédiat
	}

	filter := bson.M{field: value}
	projection := map[string]any{"id": 1}

	results, err := collection.Get(filter, projection)
	if err != nil {
		return false, nubo_error.NewInternal(err) // Erreur lors de la requête MongoDB
	}

	return len(results) > 0, nil
}
