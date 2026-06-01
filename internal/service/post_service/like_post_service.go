package post_service

import (
	"context"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

type likePayload struct {
	PostID int64 `json:"post_id"`
	UserID int64 `json:"user_id"`
}

// ToggleLike agit comme un simple routeur asynchrone ultra-rapide (Fire and Forget).
func ToggleLike(ctx context.Context, input post_models.LikePostInput) error {
	// ─────────────────────────────────────────────────────────────────────────
	// 1. IDEMPOTENCE EN RAM (O(1)) - Bloque le Spam Clic
	// ─────────────────────────────────────────────────────────────────────────
	setKey := fmt.Sprintf("post:likes_set:%d", input.PostID)

	if input.Action == "like" {
		added, err := redisgo.Rdb.SAdd(ctx, setKey, input.UserID).Result()
		if err == nil && added == 0 {
			return nil // Déjà liké, on ignore silencieusement
		}
	} else {
		removed, err := redisgo.Rdb.SRem(ctx, setKey, input.UserID).Result()
		if err == nil && removed == 0 {
			return nil // Déjà unliké, on ignore silencieusement
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. ENVOI AU WORKER ASYNCHRONE
	// ─────────────────────────────────────────────────────────────────────────
	action := redis.ActionCreate
	if input.Action == "unlike" {
		action = redis.ActionDelete
	}

	payload := likePayload{
		PostID: input.PostID,
		UserID: input.UserID,
	}

	return redis.EnqueueDB(ctx, input.UserID, 0, redis.EntityLike, action, payload, redis.TargetAll)
}
