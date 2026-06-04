package like_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/like_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
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

	// ─────────────────────────────────────────────────────────────────────────
	// 2. ENVOI AU WORKER ASYNCHRONE
	// ─────────────────────────────────────────────────────────────────────────
	action := redis.ActionCreate
	if input.Action == "unlike" {
		action = redis.ActionDelete
	}

	payload := like_models.LikePayload{
		ID:         pkg.GenerateID(),
		TargetType: 0, // ✅ 0 = Post (Polymorphisme défini)
		TargetID:   input.PostID,
		UserID:     input.UserID,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	return redis.EnqueueDB(ctx, payload.ID, 0, redis.EntityLike, action, payload, redis.TargetAll)
}
