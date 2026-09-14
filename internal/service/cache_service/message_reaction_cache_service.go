package cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// GetUserReaction récupère l'emoji spécifique d'un utilisateur sur un message
func GetUserReaction(ctx context.Context, messageID int64, userID int64) (string, error) {
	return redis.MessageUserReactions.HGet(ctx, messageID, strconv.FormatInt(userID, 10)).Result()
}

// SetUserReaction sauvegarde l'emoji choisi par l'utilisateur
func SetUserReaction(ctx context.Context, messageID int64, userID int64, emoji string) error {
	return redis.MessageUserReactions.HSet(ctx, messageID, strconv.FormatInt(userID, 10), emoji)
}

// DeleteUserReaction supprime la trace de la réaction de l'utilisateur
func DeleteUserReaction(ctx context.Context, messageID int64, userID int64) error {
	return redis.MessageUserReactions.HDel(ctx, messageID, strconv.FormatInt(userID, 10))
}

// IncrementReactionCount modifie le compteur d'un emoji spécifique
func IncrementReactionCount(ctx context.Context, messageID int64, emoji string, delta int64) error {
	key := redis.MessageReactionCounts.Key(messageID)
	newVal, err := redis.MessageReactionCounts.Client.HIncrBy(ctx, key, emoji, delta).Result()

	// Nettoyage automatique si le compteur tombe à zéro pour ne pas polluer le payload JSON
	if err == nil && newVal <= 0 {
		redis.MessageReactionCounts.Client.HDel(ctx, key, emoji)
	}
	return err
}

// GetMessageReactionCounts récupère l'ensemble des compteurs agglomérés pour un message (O(1))
func GetMessageReactionCounts(ctx context.Context, messageID int64) (map[string]int, error) {
	rawMap, err := redis.MessageReactionCounts.HGetAll(ctx, messageID).Result()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for emoji, countStr := range rawMap {
		if c, errParse := strconv.Atoi(countStr); errParse == nil && c > 0 {
			counts[emoji] = c
		}
	}
	return counts, nil
}
