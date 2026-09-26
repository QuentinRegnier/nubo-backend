package post_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DE LA TIMELINE D'UN UTILISATEUR
// ############################################################################

// GetUserPosts retourne la timeline complète d'un utilisateur.
func GetUserPosts(ctx context.Context, input post_models.GetUserPostsInput) []post_models.GetPostOutput {

	// ── ÉTAPE 1 : TENTATIVE L1 (ZSET USER CACHE) ────────────────────────────
	var postIDsList []int64
	var errCache error

	// Sauf si contournement forcé par l'utilisateur (Force=true)
	if !input.Force {
		postIDsList, errCache = cache_service.GetTopUserPostIDs(ctx, input.TargetUserID, input.Offset, input.Limit)
	}

	// Si errCache == nil et sans force, on a tapé le cache (Même si la liste est vide grâce au marqueur).
	if errCache == nil && !input.Force {
		if len(postIDsList) == 0 {
			return []post_models.GetPostOutput{} // Profil certifié vide[cite: 62]
		}
		// On délègue tout à GetPosts qui a déjà été hydraté
		return GetPosts(ctx, post_models.GetPostInput{UserID: input.CallerID, PostIDs: postIDsList})
	}

	// ── ÉTAPE 2 : FALLBACK ABSOLU L3 (POSTGRESQL) - CONTOURNEMENT MONGO ─────
	postsFromPostgres, errPg := postgres.FuncLoadUserPosts(ctx, input.TargetUserID, input.Limit, input.Offset)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Int64("target_user_id", input.TargetUserID).Msg("Échec L3 lors de la récupération de la timeline utilisateur")
		return []post_models.GetPostOutput{}
	}

	// ── ÉTAPE 3 : HYDRATATION EN TEMPS RÉEL (PROTECTION & RECONSTRUCTION) ───
	if len(postsFromPostgres) == 0 {
		// Protection anti-fantôme même en mode force : on certifie le vide à Redis.
		_ = cache_service.MarkUserTimelineEmpty(ctx, input.TargetUserID)
		return []post_models.GetPostOutput{}
	}

	// PURGE AVANT BATCH : On rase le ZSET pour écraser proprement un éventuel "-1" ou une liste corrompue.
	_ = cache_service.PurgeUserTimeline(ctx, input.TargetUserID)

	var recoveredPostIDs []int64
	for _, postPayload := range postsFromPostgres {
		// A. On reconstruit le ZSET de la timeline utilisateur de façon rectiligne.
		_ = cache_service.AddPostToUserProfile(ctx, postPayload.UserID, postPayload.ID, float64(postPayload.CreatedAt))

		// B. On blinde la RAM L1 avec les payloads complets.
		_ = object_cache_service.SetPostInObjectCache(ctx, postPayload)

		recoveredPostIDs = append(recoveredPostIDs, postPayload.ID)
	}

	// ── ÉTAPE 4 : RECYCLAGE DU SERVICE PRINCIPAL (100% CACHE HIT GARANTI) ───
	// Puisqu'on vient de pousser les objets dans l'Object Cache, GetPosts
	// lira directement la RAM pour générer la matrice et le HMAC instantanément !
	return GetPosts(ctx, post_models.GetPostInput{
		UserID:  input.CallerID,
		PostIDs: recoveredPostIDs,
	})
}
