package worker

import (
	"context"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"go.mongodb.org/mongo-driver/bson"
	libMongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func flushMongo(ctx context.Context, events []redis.AsyncEvent) {
	// Groupe par EntityType
	grouped := make(map[redis.EntityType][]redis.AsyncEvent)
	for _, e := range events {
		grouped[e.Type] = append(grouped[e.Type], e)
	}

	for entity, evts := range grouped {

		// --- RECUPERATION DU WRAPPER C (*MongoCollection) ---
		var c *mongo.MongoCollection

		switch entity {
		case redis.EntityUser:
			c = mongo.Users
		case redis.EntityUserSettings:
			c = mongo.UserSettings
		case redis.EntitySession:
			c = mongo.Sessions
		case redis.EntityRelation:
			c = mongo.Relations
		case redis.EntityPost:
			c = mongo.Posts
		case redis.EntityComment:
			c = mongo.Comments
		case redis.EntityLike:
			c = mongo.Likes
		case redis.EntityMedia:
			c = mongo.Media
		case redis.EntityConversation:
			c = mongo.ConversationsMeta
		case redis.EntityMembers:
			c = mongo.ConversationMembers
		case redis.EntityMessage:
			c = mongo.Messages
		// Ajoute ici tes autres mappings (Comments, Relations...)
		default:
			log.Printf("⚠️ Erreur: Pas de MongoCollection définie pour l'entité %s", entity)
			continue
		}

		// Sécurité : si la collection n'est pas initialisée
		if c == nil {
			log.Printf("⚠️ Erreur: La collection Mongo pour %s est nil", entity)
			continue
		}

		// --- ACCÈS AU DRIVER OFFICIEL VIA TON WRAPPER ---
		// C'est ici qu'on applique ta logique : c.DB.Collection(c.Name)
		// On suppose que c.DB est accessible (public) et c.Name aussi
		coll := c.DB.Collection(c.Name)

		// --- PREPARATION DU BULK ---
		var models []libMongo.WriteModel

		for _, e := range evts {
			switch e.Action {
			case redis.ActionCreate:
				// InsertOneModel
				models = append(models, libMongo.NewInsertOneModel().SetDocument(e.Payload))

			case redis.ActionUpdate:
				// UpdateOneModel ($set)
				models = append(models, libMongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": e.ID}).
					SetUpdate(bson.M{"$set": e.Payload}))

			case redis.ActionDelete:
				// DeleteOneModel
				models = append(models, libMongo.NewDeleteOneModel().
					SetFilter(bson.M{"_id": e.ID}))
			}
		}

		// --- EXECUTION ---
		if len(models) > 0 {
			opts := options.BulkWrite().SetOrdered(true)
			_, err := coll.BulkWrite(ctx, models, opts)
			if err != nil {
				log.Printf("❌ Erreur Mongo BulkWrite %s: %v", c.Name, err)
			}
		}
	}
}
