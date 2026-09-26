package saved_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : AJOUT / RETRAIT DES FAVORIS (TOGGLE)
// ############################################################################

// ToggleSaved gère l'ajout et le retrait d'un post aux favoris de l'utilisateur.
// Utilise le cache L1 pour bloquer instantanément les fantômes.
func ToggleSaved(ctx context.Context, userID int64, postID int64, requestedAction string) error {

	// ── ÉTAPE 1 : VÉRIFICATION D'INTÉGRITÉ DU POST ──────────────────────────

	// On vérifie que le post existe bien en L1 pour éviter de sauvegarder un post fantôme
	if !object_cache_service.IsPostInObjectCache(ctx, postID) {
		return nubo_error.NewNotFound(nubo_error.CodeNotFound, "La publication est introuvable ou indisponible.", nil)
	}

	currentTime := time.Now().UTC()
	var redisActionType redis.ActionType

	// ── ÉTAPE 2 : TRAITEMENT L1 (RAM ZSET) ──────────────────────────────────

	if requestedAction == variables.SavedActionSave {
		redisActionType = redis.ActionCreate
		zsetScore := float64(currentTime.UnixMilli())
		_ = object_cache_service.AddSavedToZSET(ctx, userID, postID, zsetScore)

	} else if requestedAction == variables.SavedActionUnsave {
		redisActionType = redis.ActionDelete
		_ = object_cache_service.RemoveSavedFromZSET(ctx, userID, postID)

	} else {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Action de sauvegarde non reconnue.", nil)
	}

	// ── ÉTAPE 3 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────

	savedRecordPayload := saved_models.SavedPayload{
		ID:        pkg.GenerateID(),
		UserID:    userID,
		PostID:    postID,
		CreatedAt: domain.TimeToMillis(currentTime),
	}

	// PartitionKey = userID pour que le shard gérant cet utilisateur centralise ses favoris
	errQueue := redis.EnqueueDB(ctx, savedRecordPayload.ID, userID, redis.EntitySaved, redisActionType, savedRecordPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("post_id", postID).Msg("Échec du Write-Behind pour ToggleSaved")
		return nubo_error.NewInternal()
	}

	return nil
}
