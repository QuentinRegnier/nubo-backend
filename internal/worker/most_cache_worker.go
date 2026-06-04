package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
)

// updateMostCache intercepte les événements pour alimenter les ZSETs (Tags, Profils, Classements)
func updateMostCache(ctx context.Context, events []redis.AsyncEvent) {
	for _, e := range events {

		// 1. SI C'EST UN NOUVEAU POST OU UNE MISE À JOUR
		if e.Type == redis.EntityPost && (e.Action == redis.ActionCreate || e.Action == redis.ActionUpdate) {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post post_models.PostPayload
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					cache_service.UpdatePostRecommendationScore(ctx, post)
					cache_service.AddPostToUserProfile(ctx, post.UserID, post.ID, post.CreatedAt.UnixMilli())
					feed_service.StoreContentVector(ctx, post)
				}
			}
		}

		// 2. SI C'EST UNE SUPPRESSION DE POST (Nettoyage ZSET)
		if e.Type == redis.EntityPost && e.Action == redis.ActionDelete {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post post_models.PostPayload
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					pipe := redisgo.Rdb.Pipeline()
					dateKey := post.CreatedAt.UTC().Format("20060102")
					pipe.ZRem(ctx, fmt.Sprintf("trend:global:daily:%s", dateKey), post.ID)

					for _, tag := range post.Hashtags {
						pipe.ZRem(ctx, fmt.Sprintf("trend:tag:%s:daily", tag), post.ID)
					}
					_, _ = pipe.Exec(ctx)
				}
			}
		}

		// 3. SI C'EST UNE INTERACTION (LIKE ou VUE agrégé)
		if (e.Type == redis.EntityLike || e.Type == redis.EntityView) && (e.Action == redis.ActionCreate || e.Action == redis.ActionDelete) {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var interactionEvent struct {
					PostID     int64 `json:"post_id"`
					TargetID   int64 `json:"target_id"`
					TargetType int   `json:"target_type"`
					UserID     int64 `json:"user_id"`
					Count      int   `json:"count"`
				}

				if err := json.Unmarshal(jsonBytes, &interactionEvent); err == nil {

					// 🛡️ BOUCLIER : Le Most Cache ignore totalement les interactions sur les commentaires
					if interactionEvent.TargetType != 0 {
						continue
					}

					targetID := interactionEvent.TargetID
					if interactionEvent.PostID != 0 {
						targetID = interactionEvent.PostID
					}

					if targetID != 0 {
						p, err := getPostWithFallback(ctx, targetID)
						if err == nil && p.Visibility != -1 {

							// VÉRIFICATION DES DROITS ASYNCHRONE
							if interactionEvent.UserID != 0 && p.UserID != interactionEvent.UserID {
								relationState := cache_service.RelationValue(ctx, p.UserID, interactionEvent.UserID)
								if relationState == -1 {
									continue
								}
								if p.Visibility == 1 && relationState < 1 {
									continue
								}
								if p.Visibility == 2 && relationState != 2 {
									continue
								}
							}

							// ÉVALUATION ALGORITHMIQUE (Plus d'écriture dans l'Object Cache ici !)
							if e.Type == redis.EntityLike {
								cache_service.EvaluatePostAfterLike(ctx, p)
							} else if e.Type == redis.EntityView {
								cache_service.EvaluatePostAfterView(ctx, p)
							}
						}
					}
				}
			}
		}

		// 4. SI C'EST UN COMMENTAIRE (Recalcul algorithmique du Post)
		if e.Type == redis.EntityComment && (e.Action == redis.ActionCreate || e.Action == redis.ActionDelete) {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var commentEvent struct {
					PostID int64 `json:"post_id"`
				}

				if err := json.Unmarshal(jsonBytes, &commentEvent); err == nil && commentEvent.PostID != 0 {
					p, err := getPostWithFallback(ctx, commentEvent.PostID)
					if err == nil && p.Visibility != -1 {
						// ÉVALUATION ALGORITHMIQUE UNIQUEMENT
						cache_service.UpdatePostRecommendationScore(ctx, p)
					}
				}
			}
		}

		updateSpeedCache(ctx, e)
	}
}

// getPostWithFallback reste inchangé
func getPostWithFallback(ctx context.Context, postID int64) (post_models.PostPayload, error) {
	var p post_models.PostPayload

	if postL1, err := object_cache_service.GetPostFromObjectCache(ctx, postID); err == nil {
		return postL1, nil
	}

	filter := map[string]any{"id": postID}
	docs, err := mongo.Posts.GetPaginated(filter, nil, 0, 1)
	if err == nil && len(docs) > 0 {
		if errStruct := pkg.ToStruct(docs[0], &p); errStruct == nil {
			_ = object_cache_service.SetPostInObjectCache(ctx, p)
			return p, nil
		}
	}

	posts, err := postgres.FuncLoadPosts([]int64{postID}, 1, 0)
	if err == nil && len(posts) > 0 {
		p = posts[0]
		_ = object_cache_service.SetPostInObjectCache(ctx, p)
		return p, nil
	}

	return post_models.PostPayload{}, fmt.Errorf("post_service %d introuvable", postID)
}
