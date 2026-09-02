package mongo

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// FullInboxResult est le type consolidé retournant les payloads complets depuis Mongo
type FullInboxResult struct {
	Conversation conversation_models.ConversationPayload
	Member       conversation_models.MemberPayload
}

// MongoLoadConversationPaginated utilise un pipeline d'agrégation pour joindre les Membres et les Conversations et trier (L2)
func MongoLoadConversationPaginated(userID int64, limit int64, offset int64) ([]FullInboxResult, error) {
	if Members == nil || Conversations == nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		// 1. Filtrer les adhésions de l'utilisateur
		{{Key: "$match", Value: bson.M{"user_id": userID}}},
		// 2. Joindre la métadonnée de conversation complète
		{{Key: "$lookup", Value: bson.M{
			"from":         "messaging.conversations",
			"localField":   "conversation_id",
			"foreignField": "id",
			"as":           "conversation_meta",
		}}},
		{{Key: "$unwind", Value: "$conversation_meta"}},
		// 3. Exclure les conversations archivées/supprimées (state != 0)
		{{Key: "$match", Value: bson.M{"conversation_meta.state": 0}}},
		// 4. Trier par le dernier message ID décroissant
		{{Key: "$sort", Value: bson.M{"conversation_meta.last_message_id": -1}}},
		// 5. Paginer
		{{Key: "$skip", Value: offset}},
		{{Key: "$limit", Value: limit}},
	}

	cursor, err := Members.DB.Collection(Members.Name).Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err := cursor.Close(ctx)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture du curseur Mongo")
		}
	}(cursor, ctx)

	var results []FullInboxResult
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err == nil {
			var fullConv conversation_models.ConversationPayload
			var fullMem conversation_models.MemberPayload

			// Le document joint contient le Full Payload de la conversation
			metaDoc := doc["conversation_meta"].(bson.M)

			// Utilisation de ToStruct pour mapper proprement la BSON en Struct complète sans erreur de type
			if errConv := pkg.ToStruct(metaDoc, &fullConv); errConv == nil {
				if errMem := pkg.ToStruct(doc, &fullMem); errMem == nil {
					results = append(results, FullInboxResult{
						Conversation: fullConv,
						Member:       fullMem,
					})
				}
			}
		}
	}
	return results, nil
}
