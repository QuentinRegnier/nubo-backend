package comment_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UpdateComment gère la modification en récupérant l'objet complet pour le Bulk Update des workers.
func UpdateComment(ctx context.Context, input comment_models.UpdateCommentInput) error {
	// ─────────────────────────────────────────────────────────────────────────
	// 1. VERIFICATION DROIT D'ACCÈS ET RÉCUPÉRATION DE L'OBJET COMPLET (Nécessaire pour les étapes suivantes)
	// ─────────────────────────────────────────────────────────────────────────

	comment, err := security_service.LeftComment(ctx, input.CommentID, input.UserID)
	if err != nil {
		return err
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. APPLICATION DES MODIFICATIONS
	// ─────────────────────────────────────────────────────────────────────────
	cleanContent := pkg.CleanStr(input.Content)
	if cleanContent == "" {
		return nubo_error.NewBadRequest("EMPTY_COMMENT", "Le commentaire ne peut pas être vide.", nil)
	}

	comment.Content = cleanContent
	comment.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// ─────────────────────────────────────────────────────────────────────────
	// 3. SAUVEGARDE ET DÉLÉGATION AUX WORKERS BATCH
	// ─────────────────────────────────────────────────────────────────────────

	// 1. Écrasement LFU immédiat
	if err := object_cache_service.SetCommentInObjectCache(ctx, comment); err != nil {
		return err
	}

	// 2. Envoi de l'objet COMPLET dans la file asynchrone pour les bulkUpdate
	return redis.EnqueueDB(ctx, comment.ID, 0, redis.EntityComment, redis.ActionUpdate, comment, redis.TargetAll)
}
