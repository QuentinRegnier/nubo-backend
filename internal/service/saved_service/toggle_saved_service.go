package saved_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// ToggleSaved gère l'ajout et le retrait d'un post aux favoris de l'utilisateur
func ToggleSaved(ctx context.Context, userID int64, postID int64, action string) error {
	// 1. Vérification que le post existe bien (L1) - Évite de sauvegarder un fantôme
	if !object_cache_service.IsPostInObjectCache(ctx, postID) {
		return nubo_error.NewNotFound("POST_NOT_FOUND", "Post introuvable ou indisponible.", nil)
	}

	now := time.Now().UTC()
	var dbAction redis.ActionType

	// 2. Traitement L1 (RAM)
	if action == "save" {
		dbAction = redis.ActionCreate
		score := float64(now.UnixMilli())
		_ = object_cache_service.AddSavedToZSET(ctx, userID, postID, score)
	} else if action == "unsave" {
		dbAction = redis.ActionDelete
		_ = object_cache_service.RemoveSavedFromZSET(ctx, userID, postID)
	} else {
		return nubo_error.NewBadRequest("INVALID_ACTION", "Action non reconnue.", nil)
	}

	// 3. Traitement L2/L3 Asynchrone (Write-Behind)
	payload := saved_models.SavedPayload{
		ID:        pkg.GenerateID(),
		UserID:    userID,
		PostID:    postID,
		CreatedAt: now,
	}

	// partitionKey = userID (le shard gérant l'utilisateur centralisera ses favoris)
	return redis.EnqueueDB(ctx, payload.ID, userID, redis.EntitySaved, dbAction, payload, redis.TargetAll)
}
