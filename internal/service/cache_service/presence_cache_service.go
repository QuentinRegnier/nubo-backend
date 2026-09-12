package cache_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// MarkUserOnline insère ou prolonge la présence de l'utilisateur dans le cache L1.
// Le TTL est strictement fixé à 90 secondes pour lisser les micro-coupures réseau
// (tolérance de 3 pings ratés de 30s)[cite: 2].
func MarkUserOnline(ctx context.Context, userID int64) error {
	// 1. On stocke une simple string "1" pour minimiser l'empreinte RAM (O(1))
	err := redis.Presence.SetPrimitive(ctx, userID, "1")
	if err != nil {
		return err
	}

	// 2. Application stricte du TTL de 90 secondes (Lissage de déconnexion)
	return redis.Expire(ctx, redis.Presence.Key(userID), 90*time.Second)
}

// IsUserOnline vérifie instantanément si un utilisateur est connecté sur n'importe quelle instance (O(1))[cite: 2].
func IsUserOnline(ctx context.Context, userID int64) bool {
	exists, _ := redis.Presence.Exists(ctx, userID)
	return exists
}
