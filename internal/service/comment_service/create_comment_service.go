package comment_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/notification_service"
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
	cleanContent := pkg.CleanStr(input.Content)
	if cleanContent == "" {
		return nubo_error.NewBadRequest("EMPTY_COMMENT", "Le commentaire ne peut pas être vide.", nil)
	}

	comment := comment_models.CommentPayload{
		ID:         pkg.GenerateID(),
		PostID:     input.PostID,
		UserID:     input.UserID,
		Content:    cleanContent, // Utilisation de la version propre
		Visibility: 0,
		LikeCount:  0,
		Score:      initialScore,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// 5. CACHE HYBRIDE (ZSET + RAM)
	var postAuthorID int64
	if object_cache_service.IsPostInObjectCache(ctx, comment.PostID) {
		_ = object_cache_service.AddCommentToZSET(ctx, comment.PostID, comment.ID, float64(comment.Score))
		_ = object_cache_service.SetCommentInObjectCache(ctx, comment)

		// === NOUVEAU : MISE À JOUR DU POST PARENT (TEMPS RÉEL) ===
		if p, err := object_cache_service.GetPostFromObjectCache(ctx, comment.PostID); err == nil {
			postAuthorID = p.UserID
			p.CommentCount += 1
			_ = object_cache_service.SetPostInObjectCache(ctx, p)
			cache_service.UpdatePostRecommendationScore(ctx, p)
		}
	}

	// 6. Envoi Asynchrone
	err := redis.EnqueueDB(ctx, comment.ID, 0, redis.EntityComment, redis.ActionCreate, comment, redis.TargetAll)

	// 7. Envoie notification (Non bloquant et sécurisé)
	if err == nil && postAuthorID != 0 {
		go func(authorID int64) {
			err := notification_service.DispatchNotification(context.Background(), authorID, input.UserID, "comment_added", comment.ID)
			if err != nil {
				logger.Log.Error().Err(err).Int64("comment_id", comment.ID).Msg("Échec de l'envoi de la notification de commentaire")
			}
		}(postAuthorID)
	}

	return err
}
