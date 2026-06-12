package comment_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

func CreateComment(ctx context.Context, input comment_models.CreateCommentInput) error {
	// 1. Temps
	now := time.Now().UTC().Format(time.RFC3339)

	// 2. Évaluation de l'autorité (SPEED Cache L1)
	priorityLevel := 0
	if userLite, err := cache_service.GetUserLite(ctx, input.UserID); err == nil {
		gradeToPriority := map[int]int{
			0: 0, // normal
			1: 1, // certifié
			2: 2, // partenaire
			3: 3, // modérateur
			4: 4, // admin
		}
		if p, ok := gradeToPriority[userLite.Grade]; ok {
			priorityLevel = p
		}
	}

	// 3. Calcul du Score de Pertinence (Likes = 0 à la création)
	initialScore := priorityLevel * 10000

	// 4. Préparation
	comment := comment_models.CommentPayload{
		ID:         pkg.GenerateID(),
		PostID:     input.PostID,
		UserID:     input.UserID,
		Content:    input.Content,
		Visibility: 0,
		LikeCount:  0,            // ✅ Doit être à 0 explicitement
		Score:      initialScore, // ✅ Injection du score calculé
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// 5. CACHE HYBRIDE (ZSET + RAM)
	if object_cache_service.IsPostInObjectCache(ctx, comment.PostID) {
		_ = object_cache_service.AddCommentToZSET(ctx, comment.PostID, comment.ID, float64(comment.Score))
		_ = object_cache_service.SetCommentInObjectCache(ctx, comment)
	}

	// 6. Envoi Asynchrone
	return redis.EnqueueDB(ctx, comment.ID, 0, redis.EntityComment, redis.ActionCreate, comment, redis.TargetAll)
}
