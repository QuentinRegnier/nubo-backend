package mongo

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"go.mongodb.org/mongo-driver/bson"
)

func MongoCheckUnique(ctx context.Context, entity redis.EntityType, field string, value any) (bool, error) {
	collection, err := Redis2Mongo(ctx, entity)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Str("entity", string(entity)).Msg("Échec de la résolution de la collection MongoDB depuis l'entité Redis")
		return false, nubo_error.NewInternal() // Coupe-circuit immédiat
	}

	filter := bson.M{field: value}
	projection := map[string]any{"id": 1}

	results, err := collection.Get(filter, projection)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
		return false, nubo_error.NewInternal() // Erreur lors de la requête MongoDB
	}

	return len(results) > 0, nil
}
