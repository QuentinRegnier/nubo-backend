package cache_service

import (
	"context"
	"fmt"
	"strconv"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
)

const (
	// Clé du Set Redis contenant les IDs des abonnés d'un utilisateur
	RedisKeySpeedFollowers = "speed:followers:%d"
)

// AddFollowerToSpeedCache ajoute l'ID de l'abonné dans le Set Redis de l'utilisateur ciblé.
// À appeler lors de la création d'une relation (abonnement/ami) validée.
func AddFollowerToSpeedCache(ctx context.Context, targetUserID int64, followerID int64) error {
	key := fmt.Sprintf(RedisKeySpeedFollowers, targetUserID)
	return redisgo.Rdb.SAdd(ctx, key, followerID).Err()
}

// RemoveFollowerFromSpeedCache retire l'ID de l'abonné du Set Redis.
// À appeler lors d'un désabonnement.
func RemoveFollowerFromSpeedCache(ctx context.Context, targetUserID int64, followerID int64) error {
	key := fmt.Sprintf(RedisKeySpeedFollowers, targetUserID)
	return redisgo.Rdb.SRem(ctx, key, followerID).Err()
}

// GetSpeedFollowers récupère la liste complète des abonnés d'un utilisateur.
// Complexité O(N) où N est le nombre d'abonnés, mais extrêmement rapide car en RAM pure.
func GetSpeedFollowers(ctx context.Context, userID int64) ([]int64, error) {
	key := fmt.Sprintf(RedisKeySpeedFollowers, userID)

	// SMembers retourne tous les éléments du Set sous forme de strings
	followerStrings, err := redisgo.Rdb.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("erreur lecture speed cache_service followers: %w", err)
	}

	followers := make([]int64, 0, len(followerStrings))
	for _, fStr := range followerStrings {
		if id, err := strconv.ParseInt(fStr, 10, 64); err == nil {
			followers = append(followers, id)
		}
	}

	return followers, nil
}
