package cache_service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// GetTopUserPostIDs récupère la timeline, gère les profils vides, et prévient les cache miss
func GetTopUserPostIDs(ctx context.Context, userID int64, offset int64, limit int64) ([]int64, error) {
	zsetKey := fmt.Sprintf("profile:posts:zset:%d", userID)

	// ✅ 1. Utilisation de l'abstrait booléen pour vérifier le Cache Miss
	exists, err := redis.Exists(ctx, zsetKey)
	if err != nil || !exists { // Vérification propre du booléen
		return nil, errors.New("cache miss")
	}

	// ✅ 2. Utilisation de l'abstrait (ZRevRange)
	idStrings, err := redis.ZRevRange(ctx, zsetKey, offset, offset+limit-1)
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

// MarkUserTimelineEmpty crée un ZSET "bouchon" avec l'ID -1 pour empêcher le martèlement de Postgres
func MarkUserTimelineEmpty(ctx context.Context, userID int64) error {
	zsetKey := fmt.Sprintf("user:posts:zset:%d", userID)

	// ✅ Adieu la structure redisgo.Z ! On passe par la méthode abstraite ZAdd
	err := redis.ZAdd(ctx, zsetKey, 0, "-1")

	// ✅ Utilisation de la primitive abstraite Expire
	_ = redis.Expire(ctx, zsetKey, 10*time.Minute)

	return err
}

// PurgeUserTimeline atomise le ZSET d'un utilisateur pour préparer une réhydratation propre
func PurgeUserTimeline(ctx context.Context, userID int64) error {
	zsetKey := fmt.Sprintf("user:posts:zset:%d", userID)
	return redis.Del(ctx, zsetKey) // ✅ C'était déjà parfait ici
}

// AddPostToUserProfile ajoute un post au ZSET de l'utilisateur avec un score chronologique
func AddPostToUserProfile(ctx context.Context, userID int64, postID int64, score float64) error {
	zsetKey := fmt.Sprintf("user:posts:zset:%d", userID)

	// ✅ SÉCURITÉ : On pulvérise l'éventuel marqueur de profil vide avant l'insertion
	_ = redis.ZRem(ctx, zsetKey, "-1")

	return redis.ZAdd(ctx, zsetKey, score, strconv.FormatInt(postID, 10))
}
