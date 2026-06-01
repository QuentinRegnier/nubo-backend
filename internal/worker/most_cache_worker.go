package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
)

// updateMostCache intercepte les événements pour alimenter les ZSETs (Tags, Profils, Classements)
func updateMostCache(ctx context.Context, events []redis.AsyncEvent) {
	for _, e := range events {

		// 1. SI C'EST UN NOUVEAU POST OU UNE MISE À JOUR
		if e.Type == redis.EntityPost && (e.Action == redis.ActionCreate || e.Action == redis.ActionUpdate) {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post models.PostRequest
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					// A. Algorithme de Recommandation (Tags, Global, Recent)
					cache_service.UpdatePostRecommendationScore(ctx, post) // Passe l'objet complet
					// B. Chronologie Utilisateur (Grille Profil) avec précision temporelle stricte
					cache_service.AddPostToUserProfile(ctx, post.UserID, post.ID, post.CreatedAt.UnixMilli())
					// C. Vecteur de Contenu pour Recommandation Personnalisée (Pilier 3)
					feed_service.StoreContentVector(ctx, post)
				}
			}
		}

		// 2. SI C'EST UNE SUPPRESSION DE POST (Nettoyage ZSET)
		if e.Type == redis.EntityPost && e.Action == redis.ActionDelete {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post models.PostRequest
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					// Utilise un Pipeline Redis pour plus de performance
					pipe := redisgo.Rdb.Pipeline()

					// A. Retrait du classement Global (Discovery)
					dateKey := post.CreatedAt.UTC().Format("20060102")
					pipe.ZRem(ctx, fmt.Sprintf("trend:global:daily:%s", dateKey), post.ID)

					// B. Retrait de tous les classements Thématiques (Tags)
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
				// STRUCTURE COMMUNE : On inclut UserID pour pouvoir vérifier les droits
				var interactionEvent struct {
					PostID   int64 `json:"post_id"`
					TargetID int64 `json:"target_id"`
					UserID   int64 `json:"user_id"` // <--- L'auteur de l'action
					Count    int   `json:"count"`
				}

				if err := json.Unmarshal(jsonBytes, &interactionEvent); err == nil {
					targetID := interactionEvent.TargetID
					if interactionEvent.PostID != 0 {
						targetID = interactionEvent.PostID
					}

					if targetID != 0 {
						p, err := getPostWithFallback(ctx, targetID)
						if err == nil && p.Visibility != -1 { // On rejette direct si Soft-Delete

							// ─────────────────────────────────────────────────────────────
							// 🛡️ VÉRIFICATION DES DROITS ASYNCHRONE
							// ─────────────────────────────────────────────────────────────
							if interactionEvent.UserID != 0 && p.UserID != interactionEvent.UserID {
								relationState := cache_service.RelationValue(ctx, p.UserID, interactionEvent.UserID)

								// Si l'utilisateur est banni ou n'a pas la relation requise,
								// on abandonne l'incrémentation (Le Hacker perd son temps).
								if relationState == -1 {
									continue
								}
								if p.Visibility == 1 && relationState < 1 { // Réservé Abonnés
									continue
								}
								if p.Visibility == 2 && relationState != 2 { // Réservé Amis
									continue
								}
							}

							// ─────────────────────────────────────────────────────────────
							// INCÉMENTATION ET MISE À JOUR DES ZSETS
							// ─────────────────────────────────────────────────────────────
							delta := 1
							if e.Action == redis.ActionDelete {
								delta = -1
							} else if interactionEvent.Count != 0 {
								delta = interactionEvent.Count
							}

							if e.Type == redis.EntityLike {
								p.LikeCount += delta
								if p.LikeCount < 0 {
									p.LikeCount = 0
								}

								_ = redis.Posts.SetObject(ctx, p.ID, p)
								cache_service.EvaluatePostAfterLike(ctx, p)

							} else if e.Type == redis.EntityView {
								p.ViewCount += delta
								if p.ViewCount < 0 {
									p.ViewCount = 0
								}

								_ = redis.Posts.SetObject(ctx, p.ID, p)
								cache_service.EvaluatePostAfterView(ctx, p)
							}
						}
					}
				}
			}
		}

		updateSpeedCache(ctx, e)
	}
}

// getPostWithFallback implémente la chaîne de résolution L1 -> L2 -> L3
// pour maximiser le taux de cache_service hit et protéger PostgreSQL lors du recalcul des scores.
func getPostWithFallback(ctx context.Context, postID int64) (models.PostRequest, error) {
	var p models.PostRequest

	// Étape 1 : Cache L1 (Redis Object Cache)
	if err := redis.Posts.GetObject(ctx, postID, &p); err == nil {
		return p, nil
	}

	// Étape 2 : Cache L2 (MongoDB - Historique récent de 30 jours)
	filter := map[string]any{"id": postID}
	docs, err := mongo.Posts.GetPaginated(filter, nil, 0, 1)
	if err == nil && len(docs) > 0 {
		if errStruct := pkg.ToStruct(docs[0], &p); errStruct == nil {
			// Réhydratation dynamique : on le remet en L1 pour les prochaines lectures
			_ = redis.Posts.SetObject(ctx, p.ID, p)
			return p, nil
		}
	}

	// Étape 3 : Base L3 (PostgreSQL - Le dernier recours)
	posts, err := postgres.FuncLoadPosts([]int64{postID}, 1, 0)
	if err == nil && len(posts) > 0 {
		p = posts[0]
		// Réhydratation dynamique : on le remet en L1
		_ = redis.Posts.SetObject(ctx, p.ID, p)
		return p, nil
	}

	return models.PostRequest{}, fmt.Errorf("post_service %d introuvable dans L1, L2 et L3", postID)
}
