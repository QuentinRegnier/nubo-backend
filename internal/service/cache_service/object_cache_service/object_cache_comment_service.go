package object_cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : OBJECT CACHE (COMMENTAIRES LFU & ZSET DE TRI)
// ############################################################################

// GetCommentFromObjectCache récupère un commentaire depuis le cache L1.
func GetCommentFromObjectCache(ctx context.Context, commentID int64) (comment_models.CommentPayload, error) {
	var commentPayload comment_models.CommentPayload

	errRedis := redis.Comments.GetObject(ctx, commentID, &commentPayload)
	if errRedis != nil {
		return comment_models.CommentPayload{}, errRedis
	}

	return commentPayload, nil
}

// SetCommentInObjectCache enregistre ou met à jour un commentaire dans le cache L1.
func SetCommentInObjectCache(ctx context.Context, commentPayload comment_models.CommentPayload) error {
	errRedis := redis.Comments.SetObject(ctx, commentPayload.ID, commentPayload)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("comment_id", commentPayload.ID).Msg("Impossible de sauvegarder le commentaire dans l'Object Cache")
		return nubo_error.NewInternal()
	}
	return nil
}

// DeleteCommentFromObjectCache purge instantanément un commentaire du cache L1.
func DeleteCommentFromObjectCache(ctx context.Context, commentID int64) error {
	errRedis := redis.Comments.DeleteObject(ctx, commentID)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Int64("comment_id", commentID).Msg("Impossible de supprimer le commentaire de l'Object Cache")
		return nubo_error.NewInternal()
	}
	return nil
}

// ############################################################################
// # GESTION DU TRI HYBRIDE DES COMMENTAIRES (ZSET L1)
// ############################################################################

// GetTopCommentIDs récupère les IDs des meilleurs commentaires depuis le ZSET (O(log(N) + M)).
func GetTopCommentIDs(ctx context.Context, postID int64, paginationOffset int64, paginationLimit int64) ([]int64, error) {
	idStringsList, errRedis := redis.PostComments.ZRevRange(ctx, postID, paginationOffset, paginationOffset+paginationLimit-1)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("post_id", postID).Msg("Erreur L1 lors de la lecture des IDs de commentaires")
		return nil, nubo_error.NewInternal()
	}

	var parsedCommentIDsList []int64
	for _, idString := range idStringsList {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			parsedCommentIDsList = append(parsedCommentIDsList, parsedID)
		}
	}

	return parsedCommentIDsList, nil
}

// AddCommentToZSET ajoute un commentaire dans le ZSET associé à une publication, plafonné par configuration.
func AddCommentToZSET(ctx context.Context, postID int64, commentID int64, commentScore float64) error {
	commentIDString := strconv.FormatInt(commentID, 10)

	errZAdd := redis.PostComments.ZAddWithCap(ctx, postID, commentScore, commentIDString, variables.MaxZsetPostComment)
	if errZAdd != nil {
		logger.Log.Error().Err(errZAdd).Int64("post_id", postID).Msg("Impossible d'ajouter le commentaire au ZSET")
		return nubo_error.NewInternal()
	}

	_ = redis.PostComments.RefreshTTL(ctx, postID)
	return nil
}

// RemoveCommentFromZSET retire un commentaire du ZSET d'une publication.
func RemoveCommentFromZSET(ctx context.Context, postID int64, commentID int64) error {
	commentIDString := strconv.FormatInt(commentID, 10)

	errZRem := redis.PostComments.ZRem(ctx, postID, commentIDString)
	if errZRem != nil {
		logger.Log.Warn().Err(errZRem).Msg("Impossible de supprimer le commentaire du ZSET")
		return nubo_error.NewInternal()
	}

	return nil
}

// IncrementCommentScoreInZSET applique un delta de score sur un commentaire dans le ZSET.
func IncrementCommentScoreInZSET(ctx context.Context, postID int64, commentID int64, scoreIncrement float64) error {
	commentIDString := strconv.FormatInt(commentID, 10)

	errIncr := redis.PostComments.ZIncrBy(ctx, postID, scoreIncrement, commentIDString)
	if errIncr != nil {
		logger.Log.Error().Err(errIncr).Msg("Impossible d'incrémenter le score du commentaire dans le ZSET")
		return nubo_error.NewInternal()
	}

	_ = redis.PostComments.RefreshTTL(ctx, postID)
	return nil
}

// PurgePostCommentsFromL1 supprime à la fois tous les payloads de commentaires de l'Object Cache
// et atomise le ZSET de tri associé à la publication.
func PurgePostCommentsFromL1(ctx context.Context, postID int64) {
	idStringsList, errRedis := redis.PostComments.ZRevRange(ctx, postID, 0, -1)
	if errRedis == nil {
		for _, idString := range idStringsList {
			if parsedCommentID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
				_ = DeleteCommentFromObjectCache(ctx, parsedCommentID)
			}
		}
	}

	// Atomisation complète du ZSET de la publication
	_ = redis.PostComments.DeleteObject(ctx, postID)
}
