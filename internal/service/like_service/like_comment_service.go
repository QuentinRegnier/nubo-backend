package like_service

import (
	"context"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/like_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// ToggleLike agit comme un routeur hybride : Tri synchrone en RAM + Persistance asynchrone.
func ToggleCommentLike(ctx context.Context, input like_models.LikeCommentInput) error {
	// ─────────────────────────────────────────────────────────────────────────
	// 1. IDEMPOTENCE EN RAM (O(1)) - Bloque le Spam Clic instantanément
	// ─────────────────────────────────────────────────────────────────────────
	if input.Action == "like" {
		if !cache_service.TryAddLikeIdempotency(ctx, 1, input.CommentID, input.UserID) { // targetType = 1
			return nil
		}
	} else {
		if !cache_service.TryRemoveLikeIdempotency(ctx, 1, input.CommentID, input.UserID) { // targetType = 1
			return nil
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. RÉCUPÉRATION EN CASCADE DU COMMENTAIRE (Pour avoir le PostID)
	// ─────────────────────────────────────────────────────────────────────────
	comment, err := getCommentCascade(ctx, input.CommentID)
	if err != nil || comment.Visibility == -1 {
		// Le commentaire a été supprimé, on annule l'idempotence au cas où et on rejette
		_ = cache_service.TryRemoveLikeIdempotency(ctx, 1, input.CommentID, input.UserID)
		return errors.New("not found")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 3. TRI SYNCHRONE (ZSET) - Le fameux Tri à Bulle Atomique !
	// ─────────────────────────────────────────────────────────────────────────
	delta := 1.0
	action := redis.ActionCreate
	if input.Action == "unlike" {
		delta = -1.0
		action = redis.ActionDelete
	}

	// On vérifie de manière opportuniste si le post est toujours Viral (en RAM)
	if object_cache_service.IsPostInObjectCache(ctx, comment.PostID) {
		// Magie Redis : Incrémentation atomique sans conflit possible
		_ = object_cache_service.IncrementCommentScoreInZSET(ctx, comment.PostID, comment.ID, delta)
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 4. DÉLÉGATION DE LA PERSISTANCE AUX WORKERS (JSON et Disque)
	// ─────────────────────────────────────────────────────────────────────────
	payload := like_models.LikePayload{
		ID:         pkg.GenerateID(),
		TargetType: 1, // ✅ 1 = Commentaire (Le worker saura quoi faire !)
		TargetID:   input.CommentID,
		UserID:     input.UserID,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	// Envoi à la file d'attente (Le counter_worker fera le +1 sur le JSON, Mongo/Postgres sur le disque)
	return redis.EnqueueDB(ctx, payload.ID, comment.PostID, redis.EntityLike, action, payload, redis.TargetAll)
}

// getCommentCascade est le fallback local ultra-rapide pour hydrater l'objet métier
func getCommentCascade(ctx context.Context, commentID int64) (comment_models.CommentPayload, error) {
	if c, err := object_cache_service.GetCommentFromObjectCache(ctx, commentID); err == nil {
		return c, nil
	}
	mongoComments, errMongo := mongo.MongoLoadComments([]int64{commentID})
	if errMongo == nil && len(mongoComments) > 0 {
		_ = object_cache_service.SetCommentInObjectCache(ctx, mongoComments[0])
		return mongoComments[0], nil
	}
	if pgComment, errPg := postgres.FuncGetComment(ctx, commentID); errPg == nil {
		_ = mongo.MongoUpsertComment(pgComment)
		_ = object_cache_service.SetCommentInObjectCache(ctx, pgComment)
		return pgComment, nil
	}
	return comment_models.CommentPayload{}, errors.New("not found")
}
