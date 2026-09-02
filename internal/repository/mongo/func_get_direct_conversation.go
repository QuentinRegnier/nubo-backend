package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// MongoGetDirectConversation interroge L2 via un pipeline d'agrégation performant.
func MongoGetDirectConversation(user1, user2 int64) (conversation_models.ConversationPayload, error) {
	if Members == nil || Conversations == nil {
		return conversation_models.ConversationPayload{}, nubo_error.NewInternal(fmt.Errorf("collections mongo non initialisées"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"user_id": bson.M{"$in": []int64{user1, user2}}}}},
		{{Key: "$group", Value: bson.M{"_id": "$conversation_id", "count": bson.M{"$sum": 1}}}},
		{{Key: "$match", Value: bson.M{"count": 2}}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "messaging.conversations",
			"localField":   "_id",
			"foreignField": "id",
			"as":           "conv",
		}}},
		{{Key: "$unwind", Value: "$conv"}},
		{{Key: "$match", Value: bson.M{"conv.type": 0, "conv.state": 0}}},
		{{Key: "$limit", Value: 1}},
	}

	cursor, err := Members.DB.Collection(Members.Name).Aggregate(ctx, pipeline)
	if err != nil {
		return conversation_models.ConversationPayload{}, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture du curseur Mongo")
		}
	}(cursor, ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil || len(results) == 0 {
		return conversation_models.ConversationPayload{}, nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation privée introuvable.", err)
	}

	// CORRECTION : Assertion de type pour forcer l'interface{} en bson.M (map[string]any)
	convMap, ok := results[0]["conv"].(bson.M)
	if !ok {
		return conversation_models.ConversationPayload{}, nubo_error.NewInternal(fmt.Errorf("impossible de formater le résultat mongo en map"))
	}

	var conv conversation_models.ConversationPayload
	if err := pkg.ToStruct(convMap, &conv); err != nil {
		return conversation_models.ConversationPayload{}, nubo_error.NewInternal(fmt.Errorf("conversion document to struct: %w", err))
	}

	return conv, nil
}
