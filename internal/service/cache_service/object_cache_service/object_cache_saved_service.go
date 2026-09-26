package object_cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : OBJECT CACHE (FAVORIS / SAVED POSTS ZSET)
// ############################################################################

// AddSavedToZSET ajoute un post dans le ZSET des favoris de l'utilisateur en L1.
func AddSavedToZSET(ctx context.Context, userID int64, postID int64, zsetScore float64) error {
	postIDString := strconv.FormatInt(postID, 10)

	errRedis := redis.Saved.ZAdd(ctx, userID, zsetScore, postIDString)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("user_id", userID).Int64("post_id", postID).Msg("Impossible d'ajouter le favori dans le ZSET L1")
		return nubo_error.NewInternal()
	}

	return nil
}

// RemoveSavedFromZSET retire un post des favoris de l'utilisateur en L1.
func RemoveSavedFromZSET(ctx context.Context, userID int64, postID int64) error {
	postIDString := strconv.FormatInt(postID, 10)

	errRedis := redis.Saved.ZRem(ctx, userID, postIDString)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Int64("user_id", userID).Int64("post_id", postID).Msg("Impossible de supprimer le favori du ZSET L1")
		return nubo_error.NewInternal()
	}

	return nil
}

// GetSavedPostIDs récupère la liste paginée des IDs de posts favoris depuis le ZSET L1.
func GetSavedPostIDs(ctx context.Context, userID int64, paginationOffset int64, paginationLimit int64) ([]int64, error) {
	idStringsList, errRedis := redis.Saved.ZRevRange(ctx, userID, paginationOffset, paginationOffset+paginationLimit-1)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("user_id", userID).Msg("Erreur L1 lors de la lecture des IDs de posts sauvegardés")
		return nil, nubo_error.NewInternal()
	}

	var parsedPostIDsList []int64
	for _, idString := range idStringsList {
		if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
			parsedPostIDsList = append(parsedPostIDsList, parsedID)
		}
	}

	return parsedPostIDsList, nil
}
