package mongo

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/repository/redis"
	"go.mongodb.org/mongo-driver/bson"
)

func MongoCheckUnique(ctx context.Context, entity redis.EntityType, field string, value any) (bool, error) {
	collection, err := redis2Mongo(ctx, entity)
	if err != nil {
		numan_log.Error(ctx).Err(err).Str("entity", string(entity)).Msg("Échec de la résolution de la collection MongoDB depuis l'entité Redis")
		return false, numan_error.NewInternal() // Coupe-circuit immédiat
	}

	filter := bson.M{field: value}
	projection := map[string]any{"id": 1}

	results, err := collection.Get(filter, projection)
	if err != nil {
		numan_log.Error(ctx).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
		return false, numan_error.NewInternal() // Erreur lors de la requête MongoDB
	}

	return len(results) > 0, nil
}
