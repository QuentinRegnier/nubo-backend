package cache_service

import (
	"context"
	"errors"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// GetTopUserPostIDs récupère la timeline, gère les profils vides, et prévient les cache miss
func GetTopUserPostIDs(ctx context.Context, userID int64, offset int64, limit int64) ([]int64, error) {
	// ✅ 1. Utilisation de l'abstrait booléen pour vérifier le Cache Miss
	exists, err := redis.UserTimeline.Exists(ctx, userID)
	if err != nil || !exists {
		return nil, errors.New("cache miss")
	}

	// ✅ 2. Utilisation de l'abstrait (ZRevRange)
	idStrings, err := redis.UserTimeline.ZRevRange(ctx, userID, offset, offset+limit-1)
	if err != nil {
		return nil, err
	}

	var ids []int64 // ✅ Initialisation explicite à un slice vide non-nil
	for _, idStr := range idStrings {
		if idStr == "-1" {
			continue // On ignore le marqueur "profil vide"
		}
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	return ids, nil
}

func MarkUserTimelineEmpty(ctx context.Context, userID int64) error {
	err := redis.UserTimeline.ZAdd(ctx, userID, 0, "-1")
	_ = redis.UserTimeline.RefreshTTL(ctx, userID)
	return err
}

func PurgeUserTimeline(ctx context.Context, userID int64) error {
	return redis.UserTimeline.DeleteObject(ctx, userID) // DeleteObject encapsule le DEL de la clé
}

func AddPostToUserProfile(ctx context.Context, userID int64, postID int64, score float64) error {
	_ = redis.UserTimeline.ZRem(ctx, userID, "-1")
	return redis.UserTimeline.ZAdd(ctx, userID, score, strconv.FormatInt(postID, 10))
}

func RemovePostFromUserProfile(ctx context.Context, userID int64, postID int64) error {
	return redis.UserTimeline.ZRem(ctx, userID, strconv.FormatInt(postID, 10))
}
