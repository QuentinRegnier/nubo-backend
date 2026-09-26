package media_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// ############################################################################
// # SERVICE : ACTIVATION / DÉSACTIVATION DES MÉDIAS (MODE FANTÔME)
// ############################################################################

// ActivateMediaBatch valide et sort une liste de médias de leur état "fantôme" (Out-of-Band).
// Appelé lorsqu'un média est rattaché à un Post, un Commentaire ou un Profil.
func ActivateMediaBatch(ctx context.Context, mediaIDs []int64, ownerID int64) error {
	if len(mediaIDs) == 0 {
		return nil
	}

	currentTime := time.Now().UTC()

	for _, mediaID := range mediaIDs {
		if mediaID <= 0 {
			continue
		}

		// ── ÉTAPE 1 : RÉCUPÉRATION DU MÉDIA (CASCADE) ───────────────────────
		// GetMediaCascade (défini dans helpers.go) s'occupe du fallback L1->L2->L3
		mediaPayload, errCascade := GetMediaCascade(ctx, mediaID)
		if errCascade != nil {
			return errCascade
		}

		// ── ÉTAPE 2 : SÉCURITÉ ZERO-TRUST ───────────────────────────────────
		if mediaPayload.OwnerID != ownerID {
			return nubo_error.NewForbidden(nubo_error.CodeForbidden, "Tentative d'utilisation d'un média qui ne vous appartient pas.", nil)
		}

		// ── ÉTAPE 3 : ACTIVATION ────────────────────────────────────────────
		mediaPayload.Visibility = true
		mediaPayload.UpdatedAt = domain.TimeToMillis(currentTime)

		// ── ÉTAPE 4 : MISE À JOUR SYNCHRONE (L1) ────────────────────────────
		_ = object_cache_service.SetMediaInObjectCache(ctx, mediaPayload)

		// ── ÉTAPE 5 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────
		errQueue := redis.EnqueueDB(ctx, mediaID, ownerID, redis.EntityMedia, redis.ActionUpdate, mediaPayload, redis.TargetAll)
		if errQueue != nil {
			logger.Log.Error().Err(errQueue).Int64("media_id", mediaID).Msg("Échec du Write-Behind lors de l'activation du média")
			return nubo_error.NewInternal()
		}
	}

	return nil
}

// DeactivateMediaBatch repasse une liste de médias à l'état de fantôme (Soft Delete).
// Le Garbage Collector (Worker) viendra les balayer physiquement et en BDD plus tard.
func DeactivateMediaBatch(ctx context.Context, mediaIDs []int64, ownerID int64) error {
	if len(mediaIDs) == 0 {
		return nil
	}

	currentTime := time.Now().UTC()

	for _, mediaID := range mediaIDs {
		if mediaID <= 0 {
			continue
		}

		// ── ÉTAPE 1 : RÉCUPÉRATION DU MÉDIA (CASCADE) ───────────────────────
		mediaPayload, errCascade := GetMediaCascade(ctx, mediaID)
		if errCascade != nil {
			return errCascade
		}

		// ── ÉTAPE 2 : SÉCURITÉ ZERO-TRUST ───────────────────────────────────
		if mediaPayload.OwnerID != ownerID {
			return nubo_error.NewForbidden(nubo_error.CodeForbidden, "Accès refusé : tentative de suppression d'un média qui ne vous appartient pas.", nil)
		}

		// ── ÉTAPE 3 : DÉSACTIVATION (MODE FANTÔME) ──────────────────────────
		mediaPayload.Visibility = false
		mediaPayload.UpdatedAt = domain.TimeToMillis(currentTime)

		// ── ÉTAPE 4 : MISE À JOUR SYNCHRONE (L1) ────────────────────────────
		_ = object_cache_service.SetMediaInObjectCache(ctx, mediaPayload)

		// ── ÉTAPE 5 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────
		errQueue := redis.EnqueueDB(ctx, mediaID, ownerID, redis.EntityMedia, redis.ActionUpdate, mediaPayload, redis.TargetAll)
		if errQueue != nil {
			logger.Log.Error().Err(errQueue).Int64("media_id", mediaID).Msg("Échec du Write-Behind lors de la désactivation du média")
			return nubo_error.NewInternal()
		}
	}

	return nil
}
