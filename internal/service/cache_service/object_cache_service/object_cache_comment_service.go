package object_cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
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

// --- GESTION DU TRI HYBRIDE (ZSET L1) ---

// GetTopCommentIDs récupère les IDs des meilleurs commentaires depuis le ZSET (O(log(N) + M))
func GetTopCommentIDs(ctx context.Context, postID int64, offset int64, limit int64) ([]int64, error) {
	idStrings, err := redis.PostComments.ZRevRange(ctx, postID, offset, offset+limit-1)
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

func AddCommentToZSET(ctx context.Context, postID int64, commentID int64, score float64) error {
	err := redis.PostComments.ZAddWithCap(ctx, postID, score, strconv.FormatInt(commentID, 10), 100)
	_ = redis.PostComments.RefreshTTL(ctx, postID)
	return err
}

func RemoveCommentFromZSET(ctx context.Context, postID int64, commentID int64) error {
	return redis.PostComments.ZRem(ctx, postID, strconv.FormatInt(commentID, 10))
}

func IncrementCommentScoreInZSET(ctx context.Context, postID int64, commentID int64, increment float64) error {
	err := redis.PostComments.ZIncrBy(ctx, postID, increment, strconv.FormatInt(commentID, 10))
	_ = redis.PostComments.RefreshTTL(ctx, postID)
	return err
}

func PurgePostCommentsFromL1(ctx context.Context, postID int64) {
	ids, err := redis.PostComments.ZRevRange(ctx, postID, 0, -1)
	if err == nil {
		for _, idStr := range ids {
			if commentID, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				_ = DeleteCommentFromObjectCache(ctx, commentID)
			}
		}
	}
	_ = redis.PostComments.DeleteObject(ctx, postID) // Atomise le ZSET
}
