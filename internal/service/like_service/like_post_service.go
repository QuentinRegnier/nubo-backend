package like_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/like_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/notification_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : AIMER OU DÉSAIMER UN POST (TOGGLE)
// ############################################################################

// TogglePostLike agit comme un routeur asynchrone ultra-rapide (Fire and Forget).
func TogglePostLike(ctx context.Context, input like_models.LikePostInput) error {

	// ── ÉTAPE 1 : IDEMPOTENCE EN RAM (O(1)) ─────────────────────────────────

	if input.Action == "like" {
		if !cache_service.TryAddLikeIdempotency(ctx, variables.LikeTargetTypePost, input.PostID, input.UserID) {
			return nil
		}
	} else {
		if !cache_service.TryRemoveLikeIdempotency(ctx, variables.LikeTargetTypePost, input.PostID, input.UserID) {
			return nil
		}
	}

	// ── ÉTAPE 2 : MISE À JOUR SYNCHRONE DU L1 (TEMPS RÉEL) ──────────────────

	deltaValue := 1
	redisActionType := redis.ActionCreate

	if input.Action == "unlike" {
		deltaValue = -1
		redisActionType = redis.ActionDelete
	}

	// Lecture opportuniste en L1
	var postAuthorID int64
	if postPayload, errCache := object_cache_service.GetPostFromObjectCache(ctx, input.PostID); errCache == nil {
		postAuthorID = postPayload.UserID
		postPayload.LikeCount += deltaValue

		if postPayload.LikeCount < 0 {
			postPayload.LikeCount = 0
		}
		_ = object_cache_service.SetPostInObjectCache(ctx, postPayload)

		// Routage intelligent vers l'IA de Classement (Most Cache)
		if input.Action == "like" {
			cache_service.EvaluatePostAfterLike(ctx, postPayload)
		}
	}

	// ── ÉTAPE 3 : DÉLÉGATION DE LA PERSISTANCE AUX WORKERS ──────────────────

	likeRecordPayload := like_models.LikePayload{
		ID:         pkg.GenerateID(),
		TargetType: variables.LikeTargetTypePost,
		TargetID:   input.PostID,
		UserID:     input.UserID,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	errQueue := redis.EnqueueDB(ctx, likeRecordPayload.ID, 0, redis.EntityLike, redisActionType, likeRecordPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("post_id", input.PostID).Msg("Échec du Write-Behind pour TogglePostLike")
		return nil // Non bloquant pour l'UX
	}

	// ── ÉTAPE 4 : NOTIFICATION TEMPS RÉEL ───────────────────────────────────

	if input.Action == "like" && postAuthorID != 0 && postAuthorID != input.UserID {
		go func(authorID int64) {
			errNotif := notification_service.DispatchNotification(context.Background(), authorID, input.UserID, variables.EventPostLiked, input.PostID)
			if errNotif != nil {
				logger.Log.Error().Err(errNotif).Int64("post_id", input.PostID).Msg("Échec de l'envoi de la notification pour un like de post")
			}
		}(postAuthorID)
	}

	return nil
}
