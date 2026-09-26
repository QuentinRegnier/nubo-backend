package comment_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// ############################################################################
// # SERVICE : MISE À JOUR DE COMMENTAIRE (ÉDITION)
// ############################################################################

// UpdateComment gère la modification en récupérant l'objet complet pour le Bulk Update des workers.
func UpdateComment(ctx context.Context, input comment_models.UpdateCommentInput) error {

	// ── ÉTAPE 1 : VÉRIFICATION DES DROITS (SÉCURITÉ) ────────────────────────

	// LeftComment s'occupe de renvoyer CodeNotFound ou CodeForbidden proprement
	commentPayload, errSecurity := security_service.LeftComment(ctx, input.CommentID, input.UserID)
	if errSecurity != nil {
		return errSecurity
	}

	// ── ÉTAPE 2 : APPLICATION DES MODIFICATIONS ─────────────────────────────

	cleanContent := pkg.CleanStr(input.Content)
	if cleanContent == "" {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Le commentaire ne peut pas être vide.", nil)
	}

	commentPayload.Content = cleanContent
	commentPayload.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// ── ÉTAPE 3 : SAUVEGARDE ET DÉLÉGATION AUX WORKERS BATCH ────────────────

	// 1. Écrasement LFU immédiat en RAM
	if errCache := object_cache_service.SetCommentInObjectCache(ctx, commentPayload); errCache != nil {
		logger.Log.Warn().Err(errCache).Int64("comment_id", commentPayload.ID).Msg("Échec de l'écrasement LFU lors de l'édition du commentaire")
	}

	// 2. Envoi de l'objet COMPLET dans la file asynchrone pour les workers (Mongo/Postgres)
	errQueue := redis.EnqueueDB(ctx, commentPayload.ID, 0, redis.EntityComment, redis.ActionUpdate, commentPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("comment_id", commentPayload.ID).Msg("Échec critique : Impossible d'enqueue la modification du commentaire")
		return nubo_error.NewInternal()
	}

	return nil
}
