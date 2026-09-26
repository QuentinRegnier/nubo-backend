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
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : CRÉATION DE COMMENTAIRE
// ############################################################################

// CreateComment gère la création d'un commentaire, l'attribution de son score
// initial selon l'autorité de l'auteur, et la mise à jour asynchrone des métriques.
func CreateComment(ctx context.Context, input comment_models.CreateCommentInput) error {

	// ── ÉTAPE 1 : NETTOYAGE ET VALIDATION DU CONTENU ────────────────────────

	cleanContent := pkg.CleanStr(input.Content)
	if cleanContent == "" {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Le commentaire ne peut pas être vide.", nil)
	}

	// ── ÉTAPE 2 : ÉVALUATION DE L'AUTORITÉ (SPEED CACHE L1) ─────────────────

	priorityLevel := 0

	if userLite, errCache := cache_service.GetUserLite(ctx, input.UserID); errCache == nil {
		if userLite.Grade >= 0 && userLite.Grade <= 4 {
			priorityLevel = userLite.Grade
		}
	}

	// ── ÉTAPE 3 : PRÉPARATION DU MODÈLE DE DONNÉES ──────────────────────────

	currentTime := time.Now().UTC().Format(time.RFC3339)
	initialScore := priorityLevel * variables.CommentPriorityMultiplier // Remplace le '10000' magique

	commentPayload := comment_models.CommentPayload{
		ID:         pkg.GenerateID(),
		PostID:     input.PostID,
		UserID:     input.UserID,
		Content:    cleanContent,
		Visibility: 0,
		LikeCount:  0,
		Score:      initialScore,
		CreatedAt:  currentTime,
		UpdatedAt:  currentTime,
	}

	// ── ÉTAPE 4 : CACHE HYBRIDE (ZSET + RAM L1) ET MISE À JOUR DU PARENT ────

	var postAuthorID int64
	if object_cache_service.IsPostInObjectCache(ctx, commentPayload.PostID) {

		// Indexation et Sauvegarde du commentaire
		_ = object_cache_service.AddCommentToZSET(ctx, commentPayload.PostID, commentPayload.ID, float64(commentPayload.Score))
		_ = object_cache_service.SetCommentInObjectCache(ctx, commentPayload)

		// Mise à jour en Temps Réel du Post Parent
		if postPayload, errPost := object_cache_service.GetPostFromObjectCache(ctx, commentPayload.PostID); errPost == nil {
			postAuthorID = postPayload.UserID
			postPayload.CommentCount += 1

			_ = object_cache_service.SetPostInObjectCache(ctx, postPayload)
			cache_service.UpdatePostRecommendationScore(ctx, postPayload)
		}
	}

	// ── ÉTAPE 5 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────

	errQueue := redis.EnqueueDB(ctx, commentPayload.ID, 0, redis.EntityComment, redis.ActionCreate, commentPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("comment_id", commentPayload.ID).Msg("Échec critique : Impossible d'enqueue la création du commentaire")
		return nubo_error.NewInternal()
	}

	// ── ÉTAPE 6 : NOTIFICATION DE L'AUTEUR DU POST (ASYNCHRONE) ─────────────

	if postAuthorID != 0 && postAuthorID != input.UserID {
		go func(authorID int64) {
			errNotif := notification_service.DispatchNotification(context.Background(), authorID, input.UserID, variables.EventCommentAdded, commentPayload.ID)
			if errNotif != nil {
				logger.Log.Error().Err(errNotif).Int64("comment_id", commentPayload.ID).Msg("Échec de l'envoi de la notification de commentaire")
			}
		}(postAuthorID)
	}

	return nil
}
