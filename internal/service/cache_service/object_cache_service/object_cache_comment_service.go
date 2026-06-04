package object_cache_service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// --- GESTION DES COMMENTAIRES (CACHE L1) ---

// GetCommentFromObjectCache récupère un commentaire depuis le cache L1
func GetCommentFromObjectCache(ctx context.Context, commentID int64) (comment_models.CommentPayload, error) {
	var c comment_models.CommentPayload
	err := redis.Comments.GetObject(ctx, commentID, &c)
	return c, err
}

// SetCommentInObjectCache enregistre ou met à jour un commentaire dans le cache L1
func SetCommentInObjectCache(ctx context.Context, comment comment_models.CommentPayload) error {
	return redis.Comments.SetObject(ctx, comment.ID, comment)
}

// DeleteCommentFromObjectCache purge instantanément un commentaire du cache L1
func DeleteCommentFromObjectCache(ctx context.Context, commentID int64) error {
	return redis.Comments.DeleteObject(ctx, commentID)
}

// GetTopCommentIDs récupère les IDs des meilleurs commentaires depuis le ZSET (O(log(N) + M))
func GetTopCommentIDs(ctx context.Context, postID int64, offset int64, limit int64) ([]int64, error) {
	zsetKey := fmt.Sprintf("object:comments:zset:%d", postID)

	idStrings, err := redisgo.Rdb.ZRevRange(ctx, zsetKey, offset, offset+limit-1).Result()
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

// AddCommentToZSET insère un commentaire à la base de l'index des meilleurs commentaires
func AddCommentToZSET(ctx context.Context, postID int64, commentID int64, score float64) error {
	zsetKey := fmt.Sprintf("object:comments:zset:%d", postID)
	// ZAdd ajoute le membre. Si le score est 0, il sera trié par ID (chronologiquement) !
	return redisgo.Rdb.ZAdd(ctx, zsetKey, redisgo.Z{Score: score, Member: strconv.FormatInt(commentID, 10)}).Err()
}

// RemoveCommentFromZSET retire un commentaire de l'index lors d'une suppression
func RemoveCommentFromZSET(ctx context.Context, postID int64, commentID int64) error {
	zsetKey := fmt.Sprintf("object:comments:zset:%d", postID)
	return redisgo.Rdb.ZRem(ctx, zsetKey, strconv.FormatInt(commentID, 10)).Err()
}

// IncrementCommentScoreInZSET ajoute ou retire 1 point de popularité
func IncrementCommentScoreInZSET(ctx context.Context, postID int64, commentID int64, increment float64) error {
	zsetKey := fmt.Sprintf("object:comments:zset:%d", postID)
	return redisgo.Rdb.ZIncrBy(ctx, zsetKey, increment, strconv.FormatInt(commentID, 10)).Err()
}
