package worker

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
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
					if interactionEvent.TargetType == 0 {
						targetID := interactionEvent.TargetID
						if interactionEvent.PostID != 0 {
							targetID = interactionEvent.PostID
						}
						if targetID != 0 {
							// Lecture opportuniste (uniquement si présent en L1)
							if p, err := object_cache_service.GetPostFromObjectCache(ctx, targetID); err == nil {
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
							}
						}
					}

					// B. INCÉMENTATION SUR UN COMMENTAIRE
					if interactionEvent.TargetType == 1 && interactionEvent.TargetID != 0 {
						// Lecture opportuniste (uniquement si présent en L1)
						if c, err := object_cache_service.GetCommentFromObjectCache(ctx, interactionEvent.TargetID); err == nil {
							c.LikeCount += delta
							if c.LikeCount < 0 {
								c.LikeCount = 0
							}
							// Sauvegarde propre
							_ = object_cache_service.SetCommentInObjectCache(ctx, c)
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
						// Sauvegarde propre
						_ = object_cache_service.SetPostInObjectCache(ctx, p)
					}
				}
			}
		}
	}
}
