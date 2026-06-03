package comment_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// CreateComment prépare le commentaire et l'envoie dans la file d'attente.
func CreateComment(ctx context.Context, input comment_models.CreateCommentInput) error {
	now := time.Now().UTC().Format(time.RFC3339)

	payload := comment_models.CommentPayload{
		ID:         pkg.GenerateID(), // PK Unique
		PostID:     input.PostID,
		UserID:     input.UserID,
		Content:    input.Content,
		Visibility: 0, // Public par défaut, ou aligné sur la visibilité du post
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Envoi asynchrone aux Workers (Postgres, Mongo, et Workers internes)
	return redis.EnqueueDB(ctx, payload.ID, 0, redis.EntityComment, redis.ActionCreate, payload, redis.TargetAll)
}
