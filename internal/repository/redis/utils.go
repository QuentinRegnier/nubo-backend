package redis

import (
	"context"
	"time"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
)

// Exists vérifie rapidement si une clé brute est présente dans le cache_service Redis (O(1))
// Renvoie true si la clé existe, false sinon.
func Exists(ctx context.Context, key string) (bool, error) {
	// La commande EXISTS de Redis renvoie le nombre de clés trouvées correspondant au nom.
	// Comme on cherche une clé unique, ça renverra 1 si elle existe, 0 sinon.
	count, err := redisgo.Rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Expire pose une durée de vie sur une clé
func Expire(ctx context.Context, key string, expiration time.Duration) error {
	return redisgo.Rdb.Expire(ctx, key, expiration).Err()
}
