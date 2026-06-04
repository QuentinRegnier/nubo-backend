package comment_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// CreateComment prépare le commentaire et l'envoie dans la file d'attente.
func CreateComment(ctx context.Context, input comment_models.CreateCommentInput) error {
	// 1. Temps
	now := time.Now().UTC().Format(time.RFC3339)

	// 2. Préparation
	comment := comment_models.CommentPayload{
		ID:         pkg.GenerateID(),
		PostID:     input.PostID,
		UserID:     input.UserID,
		Content:    input.Content,
		Visibility: 0,
		LikeCount:  0, // ✅ INITIALISATION EXPLICITE À 0
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// 3. CACHE HYBRIDE (ZSET + RAM)
	// Si le post est "viral" (déjà présent en RAM L1), on ajoute le commentaire au ZSET et en cache individuel.
	if object_cache_service.IsPostInObjectCache(ctx, comment.PostID) {
		_ = object_cache_service.AddCommentToZSET(ctx, comment.PostID, comment.ID, 0)
		_ = object_cache_service.SetCommentInObjectCache(ctx, comment)
	}

	// 4. Envoi Asynchrone
	return redis.EnqueueDB(ctx, comment.ID, 0, redis.EntityComment, redis.ActionCreate, comment, redis.TargetAll)
}
