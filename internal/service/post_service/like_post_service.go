package post_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// On aligne la structure avec ce que le Mapper attend !
type likePayload struct {
	ID         int64  `json:"id"`
	TargetType int    `json:"target_type"`
	TargetID   int64  `json:"target_id"` // Remplace post_id pour la généricité
	UserID     int64  `json:"user_id"`
	CreatedAt  string `json:"created_at"`
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
		ID:         pkg.GenerateID(), // On génère la Primary Key du Like
		TargetType: 0,                // 0 = Post (Polymorphisme)
		TargetID:   input.PostID,
		UserID:     input.UserID,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	return redis.EnqueueDB(ctx, payload.ID, 0, redis.EntityLike, action, payload, redis.TargetAll)
}
