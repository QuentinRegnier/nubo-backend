package cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// MarkUserOnline insère ou prolonge la présence de l'utilisateur dans le cache L1 (TTL: 60s)
func MarkUserOnline(ctx context.Context, userID int64) error {
	// On stocke une simple string "1" pour minimiser l'empreinte RAM (O(1))
	return redis.Presence.SetPrimitive(ctx, userID, "1")
}

// IsUserOnline vérifie instantanément si un utilisateur est connecté sur n'importe quelle instance (O(1))
func IsUserOnline(ctx context.Context, userID int64) bool {
	exists, _ := redis.Presence.Exists(ctx, userID)
	return exists
}
