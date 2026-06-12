package worker

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// updateCountersCache gère exclusivement la mise à jour des compteurs (Likes, Vues, Commentaires)
// dans l'Object Cache (L1) pour maintenir l'illusion du temps réel côté client, sans aucune Race Condition.
func updateCountersCache(ctx context.Context, events []redis.AsyncEvent) {
	for _, e := range events {

		// ─────────────────────────────────────────────────────────────────────────
		// 1. GESTION DES LIKES ET DES VUES
		// ─────────────────────────────────────────────────────────────────────────
		if (e.Type == redis.EntityLike || e.Type == redis.EntityView) && (e.Action == redis.ActionCreate || e.Action == redis.ActionDelete) {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var interactionEvent struct {
					PostID     int64 `json:"post_id"`
					TargetID   int64 `json:"target_id"`
					TargetType int   `json:"target_type"`
					Count      int   `json:"count"`
				}

				if err := json.Unmarshal(jsonBytes, &interactionEvent); err == nil {
					delta := 1
					if e.Action == redis.ActionDelete {
						delta = -1
					} else if interactionEvent.Count != 0 {
						delta = interactionEvent.Count
					}

					// A. INCÉMENTATION SUR UN POST
					if interactionEvent.TargetType == 0 && interactionEvent.TargetID != 0 {
						// Lecture opportuniste (uniquement si présent en L1)
						if p, err := object_cache_service.GetPostFromObjectCache(ctx, interactionEvent.TargetID); err == nil {
							if e.Type == redis.EntityLike {
								p.LikeCount += delta
								if p.LikeCount < 0 {
									p.LikeCount = 0
								}
							} else if e.Type == redis.EntityView {
								p.ViewCount += delta
								if p.ViewCount < 0 {
									p.ViewCount = 0
								}
							}

							// Sauvegarde propre
							_ = object_cache_service.SetPostInObjectCache(ctx, p)

							// Routage intelligent vers l'IA de Classement (Most Cache)
							if e.Type == redis.EntityLike {
								cache_service.EvaluatePostAfterLike(ctx, p)
							} else {
								cache_service.EvaluatePostAfterView(ctx, p)
							}

							// ✅ MISE À JOUR DE LA SIGNATURE VECTORIELLE (TDD §4.1)
							// On recalcule le bloc d'engagement du vecteur en asynchrone
							algorithm_service.UpdatePostEngagementVector(ctx, p)
						}
					}

					// B. INCÉMENTATION SUR UN COMMENTAIRE
					if interactionEvent.TargetType == 1 && interactionEvent.TargetID != 0 {

						// 1. On prépare une structure vierge garantie sans erreur mémoire
						var c comment_models.CommentPayload

						// 2. Tentative L1
						cFromL1, err := object_cache_service.GetCommentFromObjectCache(ctx, interactionEvent.TargetID)

						if err == nil {
							c = cFromL1
						} else {
							// ⚡ AUTO-GUÉRISON CASCADE (L1 -> L2 -> L3)
							mongoComments, errMongo := mongo.MongoLoadComments([]int64{interactionEvent.TargetID})
							if errMongo == nil && len(mongoComments) > 0 {
								c = mongoComments[0]
							} else {
								// FALLBACK ABSOLU : Le commentaire est totalement "froid"
								cPg, errPg := postgres.FuncGetComment(ctx, interactionEvent.TargetID)
								if errPg == nil {
									c = cPg
									// 🩹 Réparation immédiate de Mongo pour soulager Postgres au prochain coup
									_ = mongo.MongoUpsertComment(cPg)
								}
							}
						}

						// Si on a bien trouvé et hydraté le commentaire (en L1, L2 ou L3)
						if c.ID != 0 {
							c.LikeCount += delta
							if c.LikeCount < 0 {
								c.LikeCount = 0
							}
							c.Score += delta // MAJ mathématique directe

							// 1. Sauvegarde du JSON en RAM
							_ = object_cache_service.SetCommentInObjectCache(ctx, c)

							// 2. Frappe à la porte du Top 100.
							// On utilise AddCommentToZSET plutôt que Increment car Add gère l'insertion
							// ET applique le script Lua du Cap à 100. S'il mérite d'y être, il y sera.
							_ = object_cache_service.AddCommentToZSET(ctx, c.PostID, c.ID, float64(c.Score))
						}
					}
				}
			}
		}

		// ─────────────────────────────────────────────────────────────────────────
		// 2. GESTION DU COMPTEUR DE COMMENTAIRES SUR LE POST PARENT
		// ─────────────────────────────────────────────────────────────────────────
		if e.Type == redis.EntityComment && (e.Action == redis.ActionCreate || e.Action == redis.ActionDelete) {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var commentEvent struct {
					PostID int64 `json:"post_id"`
				}

				if err := json.Unmarshal(jsonBytes, &commentEvent); err == nil && commentEvent.PostID != 0 {
					delta := 1
					if e.Action == redis.ActionDelete {
						delta = -1
					}

					// Lecture opportuniste du Post parent
					if p, err := object_cache_service.GetPostFromObjectCache(ctx, commentEvent.PostID); err == nil {
						p.CommentCount += delta
						if p.CommentCount < 0 {
							p.CommentCount = 0
						}
						_ = object_cache_service.SetPostInObjectCache(ctx, p)

						// Un commentaire rafraîchit aussi le score global algorithmique
						cache_service.UpdatePostRecommendationScore(ctx, p)

						// ✅ MISE À JOUR DE LA SIGNATURE VECTORIELLE (TDD §4.1)
						// Le ratio commentaires/likes ayant changé, on ajuste le vecteur
						algorithm_service.UpdatePostEngagementVector(ctx, p)
					}
				}
			}
		}
	}
}
