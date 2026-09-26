package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : OBJECT CACHE (PUBLICATIONS / POSTS LFU)
// ############################################################################

// GetPostFromObjectCache récupère un post depuis le cache L1.
func GetPostFromObjectCache(ctx context.Context, postID int64) (post_models.PostPayload, error) {
	var postPayload post_models.PostPayload

	errRedis := redis.Posts.GetObject(ctx, postID, &postPayload)
	if errRedis != nil {
		return post_models.PostPayload{}, errRedis
	}

	return postPayload, nil
}

// SetPostInObjectCache enregistre ou met à jour un post dans le cache L1.
func SetPostInObjectCache(ctx context.Context, postPayload post_models.PostPayload) error {
	errRedis := redis.Posts.SetObject(ctx, postPayload.ID, postPayload)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("post_id", postPayload.ID).Msg("Impossible de sauvegarder le post dans l'Object Cache")
		return nubo_error.NewInternal()
	}
	return nil
}

// DeletePostFromObjectCache purge instantanément un post du cache L1.
func DeletePostFromObjectCache(ctx context.Context, postID int64) error {
	errRedis := redis.Posts.DeleteObject(ctx, postID)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Int64("post_id", postID).Msg("Échec de la suppression du post de l'Object Cache")
		return nubo_error.NewInternal()
	}
	return nil
}

// IsPostInObjectCache vérifie silencieusement et rapidement si un post est en RAM (O(1)).
func IsPostInObjectCache(ctx context.Context, postID int64) bool {
	isExists, errRedis := redis.Posts.Exists(ctx, postID)
	if errRedis != nil {
		return false
	}
	return isExists
}
