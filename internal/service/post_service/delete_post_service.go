package post_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// ############################################################################
// # SERVICE : RÉTRACTATION D'UNE PUBLICATION (SOFT DELETE)
// ############################################################################

// DeletePost gère la rétractation d'un post en effectuant une purge stricte et instantanée
// du Cache L1 (JSON, ZSET) et de l'IA (LSH), puis délègue le Soft Delete aux Workers.
func DeletePost(ctx context.Context, input post_models.DeletePostInput) error {

	// ── ÉTAPE 1 : CONTRÔLE D'ACCÈS ZERO-TRUST ET RÉCUPÉRATION DU POST ───────

	postPayload, errSecurity := security_service.LeftPost(ctx, input.PostID, input.UserID)
	if errSecurity != nil {
		return errSecurity // Renvoie CodeNotFound ou CodeForbidden
	}

	// ── ÉTAPE 2 : PURGE SYNCHRONE DES CACHES (DISPARITION INSTANTANÉE L1) ───

	// A. Destruction physique du Post en RAM JSON
	_ = object_cache_service.DeletePostFromObjectCache(ctx, input.PostID)

	// B. Purge de la Timeline Utilisateur (Le post disparaît du profil)
	_ = cache_service.RemovePostFromUserProfile(ctx, postPayload.UserID, input.PostID)

	// C. Purge des Commentaires en RAM associés à ce post (ZSET + JSON L1)
	object_cache_service.PurgePostCommentsFromL1(ctx, input.PostID)

	// D. Suppression Algorithmique (LSH)
	_ = algorithm_service.PurgePostVectors(ctx, input.PostID)

	// E. Purge absolue des Médias associés en RAM
	for _, mediaID := range postPayload.MediaIDs {
		_ = object_cache_service.DeleteMediaFromObjectCache(ctx, mediaID)
	}

	// ── ÉTAPE 3 : DÉLÉGATION DE LA PERSISTANCE (CASCADE BDD WRITE-BEHIND) ───

	errQueue := redis.EnqueueDB(ctx, postPayload.ID, 0, redis.EntityPost, redis.ActionDelete, postPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("post_id", input.PostID).Msg("Échec du Write-Behind lors de la suppression d'un post")
		return nubo_error.NewInternal()
	}

	return nil
}
