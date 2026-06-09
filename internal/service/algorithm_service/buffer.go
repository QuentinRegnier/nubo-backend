package algorithm_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// FeedData contient la graine et la liste des IDs pour une lettre donnée (A, B ou C)
type FeedData struct {
	Seed    int64   `json:"seed" msgpack:"seed"`
	PostIDs []int64 `json:"post_ids" msgpack:"post_ids"` // Liste triée par la caissière
	Fused   bool    `json:"fused" msgpack:"fused"`       // ✅ Indique si le panier a déjà fusionné avec ses voisins (Cas 2 consommé)
}

// FeedState est la structure interne stockée physiquement dans Redis (Remplace l'ancienne pagination)
type FeedState struct {
	GeneratedAt time.Time           `json:"generated_at" msgpack:"generated_at"`
	ActiveFeed  string              `json:"active_feed" msgpack:"active_feed"` // "A", "B" ou "C"
	Feeds       map[string]FeedData `json:"feeds" msgpack:"feeds"`
}

// GetUserFeedState récupère l'arborescence complète depuis Redis via le wrapper LFU
func GetUserFeedState(ctx context.Context, userID int64) (FeedState, error) {
	var state FeedState
	err := redis.FeedsObject.GetObject(ctx, userID, &state)
	return state, err
}

// SaveUserFeedState écrase l'état complet dans Redis
func SaveUserFeedState(ctx context.Context, userID int64, state FeedState) error {
	return redis.FeedsObject.SetObject(ctx, userID, state)
}

// DeleteUserFeedState supprime l'état (remplace l'ancien ClearBuffer)
func DeleteUserFeedState(ctx context.Context, userID int64) error {
	return redis.FeedsObject.DeleteObject(ctx, userID)
}
