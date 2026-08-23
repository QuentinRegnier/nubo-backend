package like_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/like_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/notification_service"
)

// ToggleLike agit comme un simple routeur asynchrone ultra-rapide (Fire and Forget).
func TogglePostLike(ctx context.Context, input like_models.LikePostInput) error {
	// ─────────────────────────────────────────────────────────────────────────
	// 1. IDEMPOTENCE EN RAM (O(1)) - Bloque le Spam Clic
	// ─────────────────────────────────────────────────────────────────────────
	if input.Action == "like" {
		if !cache_service.TryAddLikeIdempotency(ctx, 0, input.PostID, input.UserID) { // targetType = 0
			return nil
		}
	} else {
		if !cache_service.TryRemoveLikeIdempotency(ctx, 0, input.PostID, input.UserID) { // targetType = 0
			return nil
		}
	}

	// === NOUVEAU : MISE À JOUR SYNCHRONE DU L1 (TEMPS RÉEL) ===
	delta := 1
	action := redis.ActionCreate
	if input.Action == "unlike" {
		delta = -1
		action = redis.ActionDelete
	}

	// Lecture opportuniste en L1
	var postAuthorID int64
	if p, err := object_cache_service.GetPostFromObjectCache(ctx, input.PostID); err == nil {
		postAuthorID = p.UserID
		p.LikeCount += delta
		if p.LikeCount < 0 {
			p.LikeCount = 0
		}
		_ = object_cache_service.SetPostInObjectCache(ctx, p)

		// Routage intelligent vers l'IA de Classement (Most Cache)
		if input.Action == "like" {
			cache_service.EvaluatePostAfterLike(ctx, p)
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. ENVOI AU WORKER ASYNCHRONE
	// ─────────────────────────────────────────────────────────────────────────

	payload := like_models.LikePayload{
		ID:         pkg.GenerateID(),
		TargetType: 0, // ✅ 0 = Post (Polymorphisme défini)
		TargetID:   input.PostID,
		UserID:     input.UserID,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	err := redis.EnqueueDB(ctx, payload.ID, 0, redis.EntityLike, action, payload, redis.TargetAll)

	if err == nil && input.Action == "like" && postAuthorID != 0 {
		go func(authorID int64) {
			err := notification_service.DispatchNotification(context.Background(), authorID, input.UserID, "post_liked", input.PostID)
			if err != nil {
				fmt.Printf("TogglePostLike: failed to dispatch notification: %v\n", err)
			}
		}(postAuthorID)
	}

	return err
}
