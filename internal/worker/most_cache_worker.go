package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/vmihailenco/msgpack/v5"
	"go.mongodb.org/mongo-driver/bson"
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

					// Bucket global : retrait classique de l'ID
					pipe.ZRem(ctx, redis.TrendGlobalDaily.Key(dateKey), post.ID)

					// ✅ NOUVEAU : Désérialisation ciblée pour le retrait des tags directs
					for _, tag := range post.Hashtags {
						item := lite_models.TagPostItem{PostID: post.ID, IsIndirect: false}
						b, _ := msgpack.Marshal(item)
						pipe.ZRem(ctx, redis.TrendTagDaily.Key(fmt.Sprintf("%s:daily", tag)), string(b))
					}

					// ✅ NOUVEAU : Désérialisation ciblée pour le retrait des tags indirects
					for _, tag := range post.IndirectHashtags {
						item := lite_models.TagPostItem{PostID: post.ID, IsIndirect: true}
						b, _ := msgpack.Marshal(item)
						pipe.ZRem(ctx, redis.TrendTagDaily.Key(fmt.Sprintf("%s:daily", tag)), string(b))
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
					} else if e.Type == redis.EntityTelemetry {
						// ✅ La télémétrie remplace l'ancienne "View" : elle met à jour le leaderboard des vues ET le score global
						cache_service.EvaluatePostAfterView(ctx, p)

						// D. LE CHAÎNON MANQUANT : Mise à jour du Vecteur d'Engagement IA
						algorithm_service.UpdatePostEngagementVector(ctx, p)
					} else {
						// Pour un commentaire, on actualise juste le score global
						cache_service.UpdatePostRecommendationScore(ctx, p)
					}

					// D. LE CHAÎNON MANQUANT : Mise à jour du Vecteur d'Engagement IA
					// Garantit que le modèle vectoriel reste parfaitement isométrique avec la BDD et la RAM.
					algorithm_service.UpdatePostEngagementVector(ctx, p)
				}
			}
		}

		// 4. SI C'EST UN COMMENTAIRE (Recalcul algorithmique du Post et Extraction des Tags Indirects)
		if e.Type == redis.EntityComment && (e.Action == redis.ActionCreate || e.Action == redis.ActionDelete) {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var commentEvent struct {
					PostID  int64  `json:"post_id"`
					Content string `json:"content"` // ✅ NOUVEAU : On extrait le texte
				}
				if err := json.Unmarshal(jsonBytes, &commentEvent); err == nil && commentEvent.PostID != 0 {

					// ✅ NOUVEAU : EXTRACTION ET INJECTION (Uniquement à la création)
					if e.Action == redis.ActionCreate && commentEvent.Content != "" {
						// Expression régulière pour capturer les hashtags
						re := regexp.MustCompile(`#([\p{L}\p{N}_]+)`)
						matches := re.FindAllStringSubmatch(commentEvent.Content, -1)

						if len(matches) > 0 {
							for _, match := range matches {
								tag := match[1] // On prend le mot sans le #

								// 1. Injection L3 (Postgres Atomique)
								_ = postgres.FuncAddIndirectTagToPost(ctx, commentEvent.PostID, tag)

								// 2. Injection L2 (Mongo $addToSet garantit l'unicité)
								if mongo.Posts != nil {
									_, _ = mongo.Posts.DB.Collection(mongo.Posts.Name).UpdateOne(
										ctx,
										bson.M{"id": commentEvent.PostID},
										bson.M{"$addToSet": bson.M{"indirect_hashtags": tag}},
									)
								}

								// 3. Enregistrement L1 (Pour que le hashtag_canon le voie la nuit)
								_ = redis.Tags.SAdd(ctx, "active", tag)
							}

							// 4. On purge le Post du Cache L1 !
							// La prochaine ligne (getPostWithFallback) va forcer une lecture fraîche depuis Mongo/Postgres
							// et le recharger en RAM avec ses nouveaux tags indirects inclus.
							_ = object_cache_service.DeletePostFromObjectCache(ctx, commentEvent.PostID)
						}
					}

					// Code d'origine : on récupère le post (hydraté s'il a été purgé juste au-dessus)
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
