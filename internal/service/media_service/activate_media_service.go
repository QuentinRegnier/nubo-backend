package media_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// ActivateMediaBatch valide et sort une liste de médias de leur état "fantôme" (Out-of-Band).
func ActivateMediaBatch(ctx context.Context, mediaIDs []int64, ownerID int64) error {
	if len(mediaIDs) == 0 {
		return nil
	}

	now := time.Now().UTC()

	for _, mediaID := range mediaIDs {
		if mediaID <= 0 {
			continue
		}
		// 1. Lecture instantanée en RAM (O(1))
		mediaPayload, err := object_cache_service.GetMediaFromObjectCache(ctx, mediaID)
		if err != nil {
			return nubo_error.NewNotFound("MEDIA_NOT_FOUND", "Le média est introuvable ou a expiré.", err)
		}

		// 2. Sécurité Zero-Trust
		if mediaPayload.OwnerID != ownerID {
			return nubo_error.NewForbidden("ACCESS_DENIED", "Tentative d'utilisation d'un média qui ne vous appartient pas.", nil)
		}

		// 3. Activation
		mediaPayload.Visibility = true
		mediaPayload.UpdatedAt = domain.TimeToMillis(now)

		// 4. Mise à jour L1 et File Asynchrone
		_ = object_cache_service.SetMediaInObjectCache(ctx, mediaPayload)
		errEnqueue := redis.EnqueueDB(ctx, mediaID, ownerID, redis.EntityMedia, redis.ActionUpdate, mediaPayload, redis.TargetAll)
		if errEnqueue != nil {
			return nubo_error.NewInternal(errEnqueue)
		}
	}

	return nil
}

// DeactivateMediaBatch repasse une liste de médias à l'état de fantôme (Soft Delete).
// Le Garbage Collector viendra les balayer physiquement 24h plus tard.
func DeactivateMediaBatch(ctx context.Context, mediaIDs []int64, ownerID int64) error {
	if len(mediaIDs) == 0 {
		return nil
	}

	now := time.Now().UTC()

	for _, mediaID := range mediaIDs {
		if mediaID <= 0 {
			continue
		}
		mediaPayload, err := object_cache_service.GetMediaFromObjectCache(ctx, mediaID)
		if err != nil {
			return nubo_error.NewNotFound("MEDIA_NOT_FOUND", "Le média est introuvable ou a expiré.", err)
		}

		// Sécurité Zero-Trust
		if mediaPayload.OwnerID != ownerID {
			return nubo_error.NewForbidden("ACCESS_DENIED", "Accès refusé : tentative de suppression d'un média qui ne vous appartient pas.", nil)
		}

		// Désactivation (Mode Fantôme)
		mediaPayload.Visibility = false
		mediaPayload.UpdatedAt = domain.TimeToMillis(now)

		_ = object_cache_service.SetMediaInObjectCache(ctx, mediaPayload)
		errEnqueue := redis.EnqueueDB(ctx, mediaID, ownerID, redis.EntityMedia, redis.ActionUpdate, mediaPayload, redis.TargetAll)
		if errEnqueue != nil {
			return nubo_error.NewInternal(errEnqueue)
		}
	}

	return nil
}
