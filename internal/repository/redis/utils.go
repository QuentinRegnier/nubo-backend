package redis

import (
	"context"
	"time"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
)

// Exists vérifie rapidement si une clé brute est présente (O(1))
func Exists(ctx context.Context, key string) (bool, error) {
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
