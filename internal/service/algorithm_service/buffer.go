package algorithm_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # GESTION DE L'ÉTAT DU FEED (BUFFER)
// ############################################################################

// FeedData contient la graine et la liste des IDs ordonnés pour une lettre donnée (A, B ou C).
type FeedData struct {
	Seed    int64   `json:"seed" msgpack:"seed"`
	PostIDs []int64 `json:"post_ids" msgpack:"post_ids"` // Liste finale ordonnée par la caissière
	Fused   bool    `json:"fused" msgpack:"fused"`       // Indique si le panier a déjà fusionné avec ses voisins
}

// FeedState est la structure interne stockée physiquement dans Redis.
// Elle remplace l'ancienne pagination rigide par un modèle de tampons tournants.
type FeedState struct {
	GeneratedAt time.Time           `json:"generated_at" msgpack:"generated_at"`
	ActiveFeed  string              `json:"active_feed" msgpack:"active_feed"` // Pointeur actuel : "A", "B" ou "C"
	Feeds       map[string]FeedData `json:"feeds" msgpack:"feeds"`
}

// GetUserFeedState récupère l'arborescence complète depuis Redis via le wrapper LFU.
func GetUserFeedState(ctx context.Context, userID int64) (FeedState, error) {
	var state FeedState
	err := redis.FeedsObject.GetObject(ctx, userID, &state)
	if err != nil {
		// On sécurise l'erreur : si ça ne vient pas de Redis, c'est que la clé n'existe pas.
		return state, nubo_error.NewInternal()
	}
	return state, nil
}

// SaveUserFeedState écrase ou met à jour l'état complet dans le Speed Cache Redis.
func SaveUserFeedState(ctx context.Context, userID int64, state FeedState) error {
	err := redis.FeedsObject.SetObject(ctx, userID, state)
	if err != nil {
		return nubo_error.NewInternal()
	}
	return nil
}

// DeleteUserFeedState supprime brutalement l'état en RAM.
// Indispensable lors d'un pull-to-refresh destructif (/force) ou d'un blocage d'utilisateur.
func DeleteUserFeedState(ctx context.Context, userID int64) error {
	err := redis.FeedsObject.DeleteObject(ctx, userID)
	if err != nil {
		return nubo_error.NewInternal()
	}
	return nil
}
