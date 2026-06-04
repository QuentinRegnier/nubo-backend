package object_cache_service

import (
	"context"
	"fmt"
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
	zsetKey := fmt.Sprintf("object:comments:zset:%d", postID)

	// ✅ Utilisation de ton abstrait
	idStrings, err := redis.ZRevRange(ctx, zsetKey, offset, offset+limit-1)
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

	// ✅ Utilisation de ton abstrait (Il s'occupe de structurer redis.Z sous le capot)
	return redis.ZAdd(ctx, zsetKey, score, strconv.FormatInt(commentID, 10))
}

// RemoveCommentFromZSET retire un commentaire de l'index lors d'une suppression
func RemoveCommentFromZSET(ctx context.Context, postID int64, commentID int64) error {
	zsetKey := fmt.Sprintf("object:comments:zset:%d", postID)

	// ✅ Utilisation de la nouvelle primitive ZRem
	return redis.ZRem(ctx, zsetKey, strconv.FormatInt(commentID, 10))
}

// IncrementCommentScoreInZSET ajoute ou retire 1 point de popularité
func IncrementCommentScoreInZSET(ctx context.Context, postID int64, commentID int64, increment float64) error {
	zsetKey := fmt.Sprintf("object:comments:zset:%d", postID)

	// ✅ Utilisation de ton abstrait (Il s'occupe de caster en string sous le capot)
	return redis.ZIncrBy(ctx, zsetKey, increment, strconv.FormatInt(commentID, 10))
}

// PurgePostCommentsFromL1 supprime le ZSET et purge physiquement tous les objets JSON des commentaires associés en RAM.
func PurgePostCommentsFromL1(ctx context.Context, postID int64) {
	zsetKey := fmt.Sprintf("object:comments:zset:%d", postID)

	// 1. Récupération de tous les IDs via l'abstrait
	ids, err := redis.ZRevRange(ctx, zsetKey, 0, -1)
	if err == nil {
		for _, idStr := range ids {
			if commentID, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				// 2. Suppression physique de l'objet JSON en L1
				_ = DeleteCommentFromObjectCache(ctx, commentID)
			}
		}
	}

	// 3. Atomisation du ZSET en respectant strictement le DDD
	_ = redis.Del(ctx, zsetKey)
}
