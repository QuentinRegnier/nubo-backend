package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
)

// updateMostCache intercepte les événements pour alimenter les ZSETs (Tags, Profils, Classements)
func updateMostCache(ctx context.Context, events []redis.AsyncEvent) {
	for _, e := range events {

		// 1. SI C'EST UN NOUVEAU POST
		if e.Type == redis.EntityPost && e.Action == redis.ActionCreate {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post domain.PostRequest
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					// A. Algorithme de Recommandation (Tags, Global, Recent)
					service.UpdatePostRecommendationScore(ctx, post) // Passe l'objet complet
					// B. Chronologie Utilisateur (Grille Profil) avec précision temporelle stricte
					service.AddPostToUserProfile(ctx, post.UserID, post.ID, post.CreatedAt.UnixMilli())
					// C. Vecteur de Contenu pour Recommandation Personnalisée (Pilier 3)
					service.StoreContentVector(ctx, post)
				}
			}
		}

		// 2. SI C'EST UNE SUPPRESSION DE POST (Cache Busting)
		if e.Type == redis.EntityPost && e.Action == redis.ActionDelete {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post domain.PostRequest
				// On s'assure d'avoir bien pu extraire le UserID du payload de suppression
				if err := json.Unmarshal(jsonBytes, &post); err == nil && post.UserID != 0 {
					// Invalidation radicale : on détruit le ZSET de l'utilisateur.
					// Zéro dérive d'état garantie.
					service.InvalidateUserProfileCache(ctx, post.UserID)
				}
			}
		}

		// 3. SI C'EST UNE INTERACTION (LIKE ou VUE agrégé)
		if (e.Type == redis.EntityLike || e.Type == redis.EntityView) && e.Action == redis.ActionCreate {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				// STRUCTURE COMMUNE : Intègre le count et le drapeau d'idempotence
				var interactionEvent struct {
					TargetID              int64 `json:"target_id"`
					Count                 int   `json:"count"`
					AlreadyEvaluatedRedis bool  `json:"already_evaluated_redis"`
				}

				if err := json.Unmarshal(jsonBytes, &interactionEvent); err == nil && interactionEvent.TargetID != 0 {

					// 1. Dataloader Fallback : On récupère l'entité en tapant le moins possible sur la BDD L3
					p, err := getPostWithFallback(ctx, interactionEvent.TargetID)
					if err == nil {

						// 2. On incrémente manuellement les compteurs en RAM.
						// Cela nous évite d'attendre la synchronisation asynchrone de PostgreSQL,
						// et permet de recalculer le score immédiatement avec la nouvelle valeur.
						if e.Type == redis.EntityLike {
							p.LikeCount += interactionEvent.Count
							_ = redis.Posts.SetObject(ctx, p.ID, p) // MAJ instantanée du cache L1

							// 3. On route vers les fonctions strictes avec l'objet complet en RAM
							service.EvaluatePostAfterLike(ctx, p)
						} else if e.Type == redis.EntityView {
							p.ViewCount += interactionEvent.Count
							_ = redis.Posts.SetObject(ctx, p.ID, p) // MAJ instantanée du cache L1

							// 3. On route vers les fonctions strictes avec l'objet complet en RAM
							service.EvaluatePostAfterView(ctx, p)
						}
					}
				}
			}
		}

		updateSpeedCache(ctx, e)
	}
}

// getPostWithFallback implémente la chaîne de résolution L1 -> L2 -> L3
// pour maximiser le taux de cache hit et protéger PostgreSQL lors du recalcul des scores.
func getPostWithFallback(ctx context.Context, postID int64) (domain.PostRequest, error) {
	var p domain.PostRequest

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

	return domain.PostRequest{}, fmt.Errorf("post %d introuvable dans L1, L2 et L3", postID)
}
