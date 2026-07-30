package object_cache_service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// AddSavedToZSET ajoute un post_id au ZSET des favoris d'un utilisateur (Score = Timestamp)
func AddSavedToZSET(ctx context.Context, userID int64, postID int64, score float64) error {
	zsetKey := fmt.Sprintf("object:saved:zset:%d", userID)
	return redis.Saved.ZAdd(ctx, zsetKey, score, strconv.FormatInt(postID, 10))
}

// RemoveSavedFromZSET retire un post_id des favoris d'un utilisateur
func RemoveSavedFromZSET(ctx context.Context, userID int64, postID int64) error {
	zsetKey := fmt.Sprintf("object:saved:zset:%d", userID)
	return redis.Saved.ZRem(ctx, zsetKey, strconv.FormatInt(postID, 10))
}

// GetSavedPostIDs récupère la liste paginée des IDs sauvegardés depuis la RAM
func GetSavedPostIDs(ctx context.Context, userID int64, offset int64, limit int64) ([]int64, error) {
	zsetKey := fmt.Sprintf("object:saved:zset:%d", userID)
	idStrings, err := redis.Saved.ZRevRange(ctx, zsetKey, offset, offset+limit-1)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for _, idStr := range idStrings {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
