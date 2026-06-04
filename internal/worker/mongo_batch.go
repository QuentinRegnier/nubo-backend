package worker

import (
	"context"
	"encoding/json"
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
				if entity == redis.EntityPost || entity == redis.EntityComment { // <-- AJOUT ICI
					// SOFT DELETE pour les Posts et Commentaires
					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"id": e.ID}).
						SetUpdate(bson.M{"$set": bson.M{"visibility": -1}}))
				} else {
					// HARD DELETE pour le reste (ex: Likes, Relations)
					models = append(models, libMongo.NewDeleteOneModel().
						SetFilter(bson.M{"id": e.ID}))
				}
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
	updateCountersMongo(ctx, events)
}

// updateCountersMongo regroupe les événements et met à jour les documents Posts dans le stockage à froid L2.
func updateCountersMongo(ctx context.Context, events []redis.AsyncEvent) {
	postLikeDeltas := make(map[int64]int)
	commentLikeDeltas := make(map[int64]int) // ✅ NOUVEAU
	commentDeltas := make(map[int64]int)
	viewDeltas := make(map[int64]int)

	for _, e := range events {
		delta := 1
		if e.Action == redis.ActionDelete {
			delta = -1
		}
		jsonBytes, _ := json.Marshal(e.Payload)

		if e.Type == redis.EntityLike {
			var p struct {
				TargetType int   `json:"target_type"`
				TargetID   int64 `json:"target_id"`
			}
			_ = json.Unmarshal(jsonBytes, &p)
			if p.TargetType == 0 && p.TargetID != 0 {
				postLikeDeltas[p.TargetID] += delta // Like sur un Post
			} else if p.TargetType == 1 && p.TargetID != 0 {
				commentLikeDeltas[p.TargetID] += delta // ✅ Like sur un Commentaire
			}
		} else if e.Type == redis.EntityComment {
			var p struct {
				PostID int64 `json:"post_id"`
			}
			_ = json.Unmarshal(jsonBytes, &p)
			if p.PostID != 0 {
				commentDeltas[p.PostID] += delta
			}
		} else if e.Type == redis.EntityView {
			var p struct {
				TargetID int64 `json:"target_id"`
				Count    int   `json:"count"`
			}
			_ = json.Unmarshal(jsonBytes, &p)
			if p.TargetID != 0 {
				if p.Count != 0 {
					delta = p.Count
				}
				viewDeltas[p.TargetID] += delta
			}
		}
	}

	var postModels []libMongo.WriteModel
	var commentModels []libMongo.WriteModel // ✅ NOUVEAU

	// Modèles pour POSTS
	for id, delta := range postLikeDeltas {
		postModels = append(postModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{"$inc": bson.M{"like_count": delta}}))
	}
	for id, delta := range commentDeltas {
		postModels = append(postModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{"$inc": bson.M{"comment_count": delta}}))
	}
	for id, delta := range viewDeltas {
		postModels = append(postModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{"$inc": bson.M{"view_count": delta}}))
	}

	// Modèles pour COMMENTS
	for id, delta := range commentLikeDeltas {
		commentModels = append(commentModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{"$inc": bson.M{"like_count": delta}}))
	}

	// Exécutions indépendantes
	if len(postModels) > 0 && mongo.Posts != nil {
		_, _ = mongo.Posts.DB.Collection(mongo.Posts.Name).BulkWrite(ctx, postModels, options.BulkWrite().SetOrdered(false))
	}
	if len(commentModels) > 0 && mongo.Comments != nil {
		_, _ = mongo.Comments.DB.Collection(mongo.Comments.Name).BulkWrite(ctx, commentModels, options.BulkWrite().SetOrdered(false))
	}
}
