package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"go.mongodb.org/mongo-driver/bson"
	libMongo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func flushMongo(ctx context.Context, events []redis.AsyncEvent) {
	grouped := make(map[redis.EntityType][]redis.AsyncEvent)
	for _, e := range events {
		grouped[e.Type] = append(grouped[e.Type], e)
	}

	// Définition de la date courante pour rafraîchir le Sliding TTL de Mongo
	now := time.Now().UTC()

	for entity, evts := range grouped {
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
			c = mongo.Conversations
		case redis.EntityMembers:
			c = mongo.Members
		case redis.EntityMessage:
			c = mongo.Messages
		case redis.EntityMessageReaction:
			c = mongo.MessageReactions
		case redis.EntitySaved:
			c = mongo.Saved
		case redis.EntityNotification:
			c = mongo.Notifications
		default:
			logger.Log.Error().Interface("entity", entity).Msg("Pas de MongoCollection définie pour l'entité")
			continue
		}

		if c == nil {
			logger.Log.Error().Interface("entity", entity).Msg("La collection Mongo est nil")
			continue
		}

		coll := c.DB.Collection(c.Name)
		var models []libMongo.WriteModel

		for _, e := range evts {
			switch e.Action {
			case redis.ActionCreate:
				// Extraction en BSON pour injecter le champ `last_use`
				var doc bson.M
				dataBytes, _ := bson.Marshal(e.Payload)
				_ = bson.Unmarshal(dataBytes, &doc)
				doc["last_use"] = now
				if entity == redis.EntityMessageReaction {
					// UPSERT basé sur message_id et user_id
					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{
							"message_id": doc["message_id"],
							"user_id":    doc["user_id"],
						}).
						SetUpdate(bson.M{"$set": doc}).
						SetUpsert(true))
				} else {
					models = append(models, libMongo.NewInsertOneModel().SetDocument(doc))
				}

			case redis.ActionUpdate:
				if entity == redis.EntityRelation {
					var rel relation_models.RelationPayload
					jsonBytes, _ := json.Marshal(e.Payload)
					_ = json.Unmarshal(jsonBytes, &rel)

					// Mise à jour de la relation avec rafraîchissement du TTL
					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"primary_id": rel.PrimaryID, "secondary_id": rel.SecondaryID}).
						SetUpdate(bson.M{"$set": bson.M{
							"state":      rel.State,
							"updated_at": rel.UpdatedAt,
							"last_use":   now,
						}}))
				} else {
					// Extraction en BSON pour injecter le champ `last_use`
					var doc bson.M
					dataBytes, _ := bson.Marshal(e.Payload)
					_ = bson.Unmarshal(dataBytes, &doc)
					doc["last_use"] = now

					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"id": e.ID}). // Modification de _id en id (Snowflake)
						SetUpdate(bson.M{"$set": doc}))
				}

			case redis.ActionDelete:
				if entity == redis.EntityPost {
					var post post_models.PostPayload
					jsonBytes, _ := json.Marshal(e.Payload)
					_ = json.Unmarshal(jsonBytes, &post)

					// SOFT DELETE du Post + Refresh du TTL
					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"id": e.ID}).
						SetUpdate(bson.M{"$set": bson.M{"visibility": -1, "last_use": now}}))

					// CASCADES ASYNCHRONES
					if mongo.Comments != nil {
						_, _ = mongo.Comments.DB.Collection(mongo.Comments.Name).DeleteMany(ctx, bson.M{"post_id": e.ID})
					}
					if mongo.Likes != nil {
						_, _ = mongo.Likes.DB.Collection(mongo.Likes.Name).DeleteMany(ctx, bson.M{"target_id": e.ID, "target_type": 0})
					}
					// ✅ NOUVEAU : CASCADE HARD DELETE DES FAVORIS
					if mongo.Saved != nil {
						_, _ = mongo.Saved.DB.Collection(mongo.Saved.Name).DeleteMany(ctx, bson.M{"post_id": e.ID})
					}
					if mongo.Media != nil && len(post.MediaIDs) > 0 {
						models = append(models, libMongo.NewDeleteManyModel().
							SetFilter(bson.M{"id": bson.M{"$in": post.MediaIDs}}))
					}

					// ✅ NOUVEAU BLOC : CASCADE POUR LES MESSAGES
				} else if entity == redis.EntityMessage {
					// SOFT DELETE du Message + Refresh du TTL
					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"id": e.ID}).
						SetUpdate(bson.M{"$set": bson.M{"visibility": false, "last_use": now}}))

					// CASCADE HARD DELETE DES RÉACTIONS ASSOCIÉES
					if mongo.MessageReactions != nil {
						_, _ = mongo.MessageReactions.DB.Collection(mongo.MessageReactions.Name).DeleteMany(ctx, bson.M{"message_id": e.ID})
					}

				} else if entity == redis.EntityComment {
					// SOFT DELETE pour les Commentaires effacés unitairement + Refresh du TTL
					models = append(models, libMongo.NewUpdateOneModel().
						SetFilter(bson.M{"id": e.ID}).
						SetUpdate(bson.M{"$set": bson.M{"visibility": -1, "last_use": now}}))
				} else if entity == redis.EntityRelation {
					// HARD DELETE (On n'a pas besoin de TTL sur un élément détruit)
					var rel relation_models.RelationPayload
					jsonBytes, _ := json.Marshal(e.Payload)
					_ = json.Unmarshal(jsonBytes, &rel)
					models = append(models, libMongo.NewDeleteOneModel().SetFilter(bson.M{"primary_id": rel.PrimaryID, "secondary_id": rel.SecondaryID}))
				} else if entity == redis.EntityMessageReaction {
					// DELETE basé sur message_id et user_id (Payload requis)
					var doc bson.M
					dataBytes, _ := bson.Marshal(e.Payload)
					_ = bson.Unmarshal(dataBytes, &doc)
					models = append(models, libMongo.NewDeleteOneModel().
						SetFilter(bson.M{
							"message_id": doc["message_id"],
							"user_id":    doc["user_id"],
						}))
				} else if entity == redis.EntitySaved {
					// HARD DELETE
					var sav saved_models.SavedPayload
					jsonBytes, _ := json.Marshal(e.Payload)
					_ = json.Unmarshal(jsonBytes, &sav)
					models = append(models, libMongo.NewDeleteOneModel().
						SetFilter(bson.M{"user_id": sav.UserID, "post_id": sav.PostID}))
				} else {
					// HARD DELETE
					models = append(models, libMongo.NewDeleteOneModel().
						SetFilter(bson.M{"id": e.ID}))
				}
			}
		}

		if len(models) > 0 {
			opts := options.BulkWrite().SetOrdered(true)
			_, err := coll.BulkWrite(ctx, models, opts)
			if err != nil {
				logger.Log.Error().Err(err).Str("collection", c.Name).Msg("Erreur Mongo BulkWrite")
			}
		}
	}

	updateCountersMongo(ctx, events)
}

func updateCountersMongo(ctx context.Context, events []redis.AsyncEvent) {
	postLikeDeltas := make(map[int64]int)
	commentLikeDeltas := make(map[int64]int)
	commentDeltas := make(map[int64]int)
	viewDeltas := make(map[int64]int)
	reportDeltas := make(map[int64]int)
	messageReactionDeltas := make(map[int64]map[string]int)

	telemetryDwellSum := make(map[int64]float64)
	telemetryDwellSq := make(map[int64]float64)
	telemetryClicks := make(map[int64]int)

	for _, e := range events {
		delta := 1
		if e.Action == redis.ActionDelete {
			delta = -1
		}

		jsonBytes, err := json.Marshal(e.Payload)
		if err != nil {
			logger.Log.Error().Err(err).Interface("payload", e.Payload).Msg("Erreur Marshal Payload dans updateCountersMongo")
			continue
		}

		if e.Type == redis.EntityLike {
			var p struct {
				TargetType int   `json:"target_type"`
				TargetID   int64 `json:"target_id"`
			}
			if err := json.Unmarshal(jsonBytes, &p); err == nil {
				if p.TargetType == 0 && p.TargetID != 0 {
					postLikeDeltas[p.TargetID] += delta
				} else if p.TargetType == 1 && p.TargetID != 0 {
					commentLikeDeltas[p.TargetID] += delta
				}
			}
		} else if e.Type == redis.EntityComment {
			var p struct {
				PostID int64 `json:"post_id"`
			}
			if err := json.Unmarshal(jsonBytes, &p); err == nil && p.PostID != 0 {
				commentDeltas[p.PostID] += delta
			}
		} else if e.Type == redis.EntityView {
			var p struct {
				TargetID int64 `json:"target_id"`
				Count    int   `json:"count"`
			}
			if err := json.Unmarshal(jsonBytes, &p); err == nil && p.TargetID != 0 {
				if p.Count != 0 {
					delta = p.Count
				}
				viewDeltas[p.TargetID] += delta
			}
		} else if e.Type == redis.EntityReport {
			var r struct {
				TargetType int     `json:"target_type"`
				TargetIDs  []int64 `json:"target_ids"`
			}
			if err := json.Unmarshal(jsonBytes, &r); err == nil && r.TargetType == 1 {
				for _, id := range r.TargetIDs {
					reportDeltas[id] += delta
				}
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
				// ✅ NOUVEAU : On comptabilise automatiquement une vue classique
				viewDeltas[t.PostID] += 1

				dwellVal := float64(t.DwellTimeMs)
				telemetryDwellSum[t.PostID] += dwellVal
				telemetryDwellSq[t.PostID] += dwellVal * dwellVal

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
		} else if e.Type == redis.EntityMessage && e.Action == redis.ActionBuild {
			var p struct {
				MessageID int64          `json:"message_id"`
				Deltas    map[string]int `json:"deltas"`
			}
			if err := json.Unmarshal(jsonBytes, &p); err == nil && p.MessageID != 0 {
				if messageReactionDeltas[p.MessageID] == nil {
					messageReactionDeltas[p.MessageID] = make(map[string]int)
				}
				for emoji, d := range p.Deltas {
					messageReactionDeltas[p.MessageID][emoji] += d
				}
			}
		}
	}

	var postModels []libMongo.WriteModel
	var commentModels []libMongo.WriteModel
	var messageModels []libMongo.WriteModel

	now := time.Now().UTC()

	for id, delta := range postLikeDeltas {
		postModels = append(postModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{
			"$inc": bson.M{"like_count": delta},
			"$set": bson.M{"last_use": now},
		}))
	}

	for id, delta := range commentLikeDeltas {
		commentModels = append(commentModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{
			"$inc": bson.M{"like_count": delta, "score": delta},
			"$set": bson.M{"last_use": now},
		}))
	}

	for id, delta := range commentDeltas {
		postModels = append(postModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{
			"$inc": bson.M{"comment_count": delta},
			"$set": bson.M{"last_use": now},
		}))
	}

	for id, delta := range viewDeltas {
		postModels = append(postModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{
			"$inc": bson.M{"view_count": delta},
			"$set": bson.M{"last_use": now},
		}))
	}

	for id, delta := range reportDeltas {
		postModels = append(postModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{
			"$inc": bson.M{"report_count": delta},
			"$set": bson.M{"last_use": now},
		}))
	}

	for id, sum := range telemetryDwellSum {
		postModels = append(postModels, libMongo.NewUpdateOneModel().SetFilter(bson.M{"id": id}).SetUpdate(bson.M{
			"$inc": bson.M{
				"telemetry_dwell_sum": sum,
				"telemetry_dwell_sq":  telemetryDwellSq[id],
				"telemetry_clicks":    telemetryClicks[id],
			},
			"$set": bson.M{"last_use": now},
		}))
	}

	for msgID, deltas := range messageReactionDeltas {
		incMap := bson.M{}
		for emoji, d := range deltas {
			if d != 0 {
				incMap["attachments.reaction_counts."+emoji] = d
			}
		}

		if len(incMap) > 0 {
			messageModels = append(messageModels, libMongo.NewUpdateOneModel().
				SetFilter(bson.M{"id": msgID}).
				SetUpdate(bson.M{
					"$inc": incMap,
					"$set": bson.M{"last_use": now},
				}))
		}
	}

	if len(postModels) > 0 && mongo.Posts != nil {
		if _, err := mongo.Posts.DB.Collection(mongo.Posts.Name).BulkWrite(ctx, postModels, options.BulkWrite().SetOrdered(false)); err != nil {
			logger.Log.Error().Err(err).Msg("Erreur BulkWrite Posts Mongo")
		}
	}
	if len(commentModels) > 0 && mongo.Comments != nil {
		if _, err := mongo.Comments.DB.Collection(mongo.Comments.Name).BulkWrite(ctx, commentModels, options.BulkWrite().SetOrdered(false)); err != nil {
			logger.Log.Error().Err(err).Msg("Erreur BulkWrite Comments Mongo")
		}
	}
	if len(messageModels) > 0 && mongo.Messages != nil {
		if _, err := mongo.Messages.DB.Collection(mongo.Messages.Name).BulkWrite(ctx, messageModels, options.BulkWrite().SetOrdered(false)); err != nil {
			logger.Log.Error().Err(err).Msg("Erreur BulkWrite Messages Mongo")
		}
	}
}
