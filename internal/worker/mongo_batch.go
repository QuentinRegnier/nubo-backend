package worker

import (
	"context"
	"encoding/json"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
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
		case redis.EntitySaved:
			c = mongo.Saved
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
				if entity == redis.EntityRelation {
					// UPDATE par clé composite pour les relations
					var rel relation_models.RelationPayload
					jsonBytes, _ := json.Marshal(e.Payload)
					_ = json.Unmarshal(jsonBytes, &rel)

					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"primary_id": rel.PrimaryID, "secondary_id": rel.SecondaryID}).
						SetUpdate(bson.M{"$set": bson.M{"state": rel.State, "updated_at": rel.UpdatedAt}}))
				} else {
					// Update classique par ID
					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"_id": e.ID}).
						SetUpdate(bson.M{"$set": e.Payload}))
				}

			// ... dans la boucle switch e.Action de flushMongo ...

			case redis.ActionDelete:
				if entity == redis.EntityPost {
					// On doit décoder le payload pour récupérer la liste des MediaIDs
					var post post_models.PostPayload
					jsonBytes, _ := json.Marshal(e.Payload)
					_ = json.Unmarshal(jsonBytes, &post)

					// 1. SOFT DELETE du Post
					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"id": e.ID}).
						SetUpdate(bson.M{"$set": bson.M{"visibility": -1}}))

					// 2. HARD DELETE des Commentaires
					if mongo.Comments != nil {
						_, _ = mongo.Comments.DB.Collection(mongo.Comments.Name).DeleteMany(ctx, bson.M{"post_id": e.ID})
					}

					// 3. HARD DELETE des Likes
					if mongo.Likes != nil {
						_, _ = mongo.Likes.DB.Collection(mongo.Likes.Name).DeleteMany(ctx, bson.M{"target_id": e.ID, "target_type": 0})
					}

					// ✅ 4. HARD DELETE des Médias dans Mongo
					if mongo.Media != nil && len(post.MediaIDs) > 0 {
						models = append(models, libMongo.NewDeleteManyModel().
							SetFilter(bson.M{"id": bson.M{"$in": post.MediaIDs}}))
					}
				} else if entity == redis.EntityComment {
					// SOFT DELETE pour les Commentaires effacés unitairement
					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"id": e.ID}).
						SetUpdate(bson.M{"$set": bson.M{"visibility": -1}}))
				} else if entity == redis.EntityRelation {
					// HARD DELETE pour les Relations effacées unitairement
					var rel relation_models.RelationPayload
					jsonBytes, _ := json.Marshal(e.Payload)
					_ = json.Unmarshal(jsonBytes, &rel)

					models = append(models, libMongo.NewDeleteOneModel().SetFilter(bson.M{"primary_id": rel.PrimaryID, "secondary_id": rel.SecondaryID}))
				} else if entity == redis.EntitySaved {
					// HARD DELETE des favoris
					var sav saved_models.SavedPayload
					jsonBytes, _ := json.Marshal(e.Payload)
					_ = json.Unmarshal(jsonBytes, &sav)

					models = append(models, libMongo.NewDeleteOneModel().
						SetFilter(bson.M{"user_id": sav.UserID, "post_id": sav.PostID}))
				} else {
					// HARD DELETE pour les autres entités
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
	telemetryDwellSum := make(map[int64]float64)
	telemetryDwellSq := make(map[int64]float64)
	telemetryClicks := make(map[int64]int)

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
		} else if e.Type == redis.EntityTelemetry {
			var t struct {
				PostID       int64 `json:"post_id"`
				DwellTimeMs  int   `json:"dwell_time_ms"`
				IsClicked    bool  `json:"is_clicked"`
				DeepScroll   bool  `json:"deep_scroll"`
				ProfileVisit bool  `json:"profile_visit"`
			}
			if err := json.Unmarshal(jsonBytes, &t); err == nil && t.PostID != 0 {
				dwellVal := float64(t.DwellTimeMs)
				telemetryDwellSum[t.PostID] += dwellVal
				telemetryDwellSq[t.PostID] += (dwellVal * dwellVal)

				clicks := 0
				if t.IsClicked {
					clicks++
				}
				if t.DeepScroll {
					clicks++
				}
				if t.ProfileVisit {
					clicks++
				}

				telemetryClicks[t.PostID] += clicks
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
	for id, delta := range commentLikeDeltas {
		commentModels = append(commentModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{"$inc": bson.M{"like_count": delta, "score": delta}}))
	}
	for id, sum := range telemetryDwellSum {
		postModels = append(postModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{"$inc": bson.M{"telemetry_dwell_sum": sum, "telemetry_dwell_sq": telemetryDwellSq[id], "telemetry_clicks": telemetryClicks[id]}}))
	}

	// Exécutions indépendantes
	if len(postModels) > 0 && mongo.Posts != nil {
		_, _ = mongo.Posts.DB.Collection(mongo.Posts.Name).BulkWrite(ctx, postModels, options.BulkWrite().SetOrdered(false))
	}
	if len(commentModels) > 0 && mongo.Comments != nil {
		_, _ = mongo.Comments.DB.Collection(mongo.Comments.Name).BulkWrite(ctx, commentModels, options.BulkWrite().SetOrdered(false))
	}
}
