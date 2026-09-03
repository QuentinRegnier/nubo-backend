package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
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
					// Uniquement de l'algorithmique (Global & Personnalisé)
					cache_service.UpdatePostRecommendationScore(ctx, post)
					algorithm_service.StoreContentVector(ctx, post)
				}
			}
		}

		// 2. SI C'EST UNE SUPPRESSION DE POST (Nettoyage ZSET via Pipeline Découplé L1)
		if e.Type == redis.EntityPost && e.Action == redis.ActionDelete {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post post_models.PostPayload
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					pipe := redis.TrendGlobalDaily.Pipeline()
					dateKey := post.CreatedAt.UTC().Format("20060102")
					pipe.ZRem(ctx, redis.TrendGlobalDaily.Key(dateKey), post.ID)

					for _, tag := range post.Hashtags {
						pipe.ZRem(ctx, redis.TrendTagDaily.Key(fmt.Sprintf("%s:daily", tag)), post.ID)
					}
					_, _ = pipe.Exec(ctx)
				}
			}
		}

		// 3. INTERACTIONS : Likes, Vues, Commentaires ET Télémétrie
		if e.Type == redis.EntityLike || e.Type == redis.EntityView || e.Type == redis.EntityComment || e.Type == redis.EntityTelemetry {
			jsonBytes, err := json.Marshal(e.Payload)
			if err != nil {
				continue
			}

			var targetPostID int64
			var callerID int64

			// A. Extraction sécurisée et explicite de l'ID selon le type d'événement
			if e.Type == redis.EntityLike || e.Type == redis.EntityView {
				var payload struct {
					PostID     int64 `json:"post_id"`
					TargetID   int64 `json:"target_id"`
					TargetType int   `json:"target_type"`
					UserID     int64 `json:"user_id"`
				}
				if json.Unmarshal(jsonBytes, &payload) == nil {
					if payload.TargetType != 0 {
						continue // 🛡️ BOUCLIER : Ignore les intéractions sur les commentaires
					}
					targetPostID = payload.TargetID
					if payload.PostID != 0 {
						targetPostID = payload.PostID
					}
					callerID = payload.UserID
				}
			} else if e.Type == redis.EntityComment || e.Type == redis.EntityTelemetry {
				var payload struct {
					PostID int64 `json:"post_id"`
					UserID int64 `json:"user_id"` // Facultatif pour le commentaire, présent pour la télémétrie
				}
				if json.Unmarshal(jsonBytes, &payload) == nil {
					targetPostID = payload.PostID
					callerID = payload.UserID
				}
			}

			// B. Exécution unifiée de l'hydratation et du recalcul
			if targetPostID != 0 {
				p, err := getPostWithFallback(ctx, targetPostID)
				if err == nil && p.Visibility != -1 {

					// VÉRIFICATION DES DROITS (Uniquement si un CallerID est identifié)
					if callerID != 0 && p.UserID != callerID {
						relationState := cache_service.RelationValue(ctx, p.UserID, callerID)
						if relationState == -1 || (p.Visibility == 1 && relationState < 1) || (p.Visibility == 2 && relationState != 2) {
							continue
						}
					}

					// C. ROUTAGE SCALAIRE (Mise à jour des ZSETs et du score global de recommandation)
					if e.Type == redis.EntityLike {
						cache_service.EvaluatePostAfterLike(ctx, p)
					} else if e.Type == redis.EntityView {
						cache_service.EvaluatePostAfterView(ctx, p)
					} else {
						// Pour un commentaire ou de la télémétrie, on actualise le score global
						cache_service.UpdatePostRecommendationScore(ctx, p)
					}

					// D. LE CHAÎNON MANQUANT : Mise à jour du Vecteur d'Engagement IA
					// Garantit que le modèle vectoriel reste parfaitement isométrique avec la BDD et la RAM.
					algorithm_service.UpdatePostEngagementVector(ctx, p)
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

	return post_models.PostPayload{}, nubo_error.NewNotFound("POST_NOT_FOUND", "Post introuvable", nil)
}
