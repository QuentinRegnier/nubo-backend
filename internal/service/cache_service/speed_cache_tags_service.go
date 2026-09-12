package cache_service

import (
	"context"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// SearchTagsByPrefix recherche des hashtags en O(log(N)) RAM
func SearchTagsByPrefix(ctx context.Context, prefix string, limit int64) ([]string, error) {
	// 1. Recherche ultra-rapide dans l'index lexicographique
	lexResults, err := redis.TagsLex.ZRangeByLex(ctx, "lex", strings.ToLower(prefix), limit)
	if err != nil {
		return nil, err
	}

	if len(lexResults) == 0 {
		return []string{}, nil
	}

	// 2. Formatage du retour
	// Pour les tags, le slug est la valeur stockée directement, pas besoin de split ou d'ID.
	var tags []string
	for _, res := range lexResults {
		tags = append(tags, res)
	}

	return tags, nil
}

// StoreTagInSpeedCache (Optionnel, à appeler depuis le hashtag_canon worker quand un nouveau tag naît)
func StoreTagInSpeedCache(ctx context.Context, tag string) error {
	return redis.TagsLex.ZAdd(ctx, "lex", 0, strings.ToLower(tag))
}

// IsTagTrending vérifie en O(1) dans la RAM si un tag fait partie du Top des tendances mondiales.
func IsTagTrending(ctx context.Context, tag string) bool {
	// Le Leaderboard stocke les tags avec leur score de hype
	_, err := redis.HashtagLeaderboard.Client.ZScore(ctx, redis.HashtagLeaderboard.Key("global"), tag).Result()
	return err == nil
}
