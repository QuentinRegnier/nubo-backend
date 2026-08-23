package object_cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

func AddSavedToZSET(ctx context.Context, userID int64, postID int64, score float64) error {
	return redis.Saved.ZAdd(ctx, userID, score, strconv.FormatInt(postID, 10))
}

func RemoveSavedFromZSET(ctx context.Context, userID int64, postID int64) error {
	return redis.Saved.ZRem(ctx, userID, strconv.FormatInt(postID, 10))
}

func GetSavedPostIDs(ctx context.Context, userID int64, offset int64, limit int64) ([]int64, error) {
	idStrings, err := redis.Saved.ZRevRange(ctx, userID, offset, offset+limit-1)
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
