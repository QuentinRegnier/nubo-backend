package post_service

import (
	"context"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetUserPosts retourne la timeline complète d'un utilisateur
func GetUserPosts(ctx context.Context, input post_models.GetUserPostsInput) []post_models.GetPostOutput {

	// ─────────────────────────────────────────────────────────────────────────
	// 1. TENTATIVE L1 (ZSET User Cache) - Sauf si contournement forcé
	// ─────────────────────────────────────────────────────────────────────────
	var ids []int64
	var errCache error

	if input.Force {
		errCache = errors.New("forced fallback") // Déclenche artificiellement le fallback
	} else {
		ids, errCache = cache_service.GetTopUserPostIDs(ctx, input.TargetUserID, input.Offset, input.Limit)
	}

	// Si errCache == nil, on a tapé le cache (Même si ids est vide grâce à notre marqueur -1 !)
	if errCache == nil {
		if len(ids) == 0 {
			return []post_models.GetPostOutput{} // Profil certifié vide
		}
		// ✨ MAGIE : On délègue tout à GetPosts qui a déjà été hydraté
		return GetPosts(ctx, post_models.GetPostInput{UserID: input.CallerID, PostIDs: ids})
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. FALLBACK ABSOLU L3 (Postgres) - Contournement Mongo
	// ─────────────────────────────────────────────────────────────────────────
	posts, errPg := postgres.FuncLoadUserPosts(ctx, input.TargetUserID, input.Limit, input.Offset)
	if errPg != nil {
		return []post_models.GetPostOutput{}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 3. HYDRATATION EN TEMPS RÉEL (Protection & Reconstruction Propre)
	// ─────────────────────────────────────────────────────────────────────────
	if len(posts) == 0 {
		// ✅ Protection anti-fantôme même en mode force : on certifie le vide à Redis
		_ = cache_service.MarkUserTimelineEmpty(ctx, input.TargetUserID)
		return []post_models.GetPostOutput{}
	}

	// ✅ PURGE AVANT BATCH : On rase le ZSET pour écraser proprement un éventuel "-1" ou une liste corrompue
	_ = cache_service.PurgeUserTimeline(ctx, input.TargetUserID)

	var postIDs []int64
	for _, p := range posts {
		// A. On reconstruit le ZSET de la timeline utilisateur de façon rectiligne
		_ = cache_service.AddPostToUserProfile(ctx, p.UserID, p.ID, float64(p.CreatedAt.UnixMilli()))
		// B. On blinde la RAM L1 avec les payloads complets
		_ = object_cache_service.SetPostInObjectCache(ctx, p)

		postIDs = append(postIDs, p.ID)
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 4. RECYCLAGE DU SERVICE PRINCIPAL (100% Cache HIT garanti)
	// ─────────────────────────────────────────────────────────────────────────
	// Puisqu'on vient de pousser les objets dans l'Object Cache, GetPosts
	// lira directement la RAM pour générer la matrice et le HMAC instantanément !
	return GetPosts(ctx, post_models.GetPostInput{
		UserID:  input.CallerID,
		PostIDs: postIDs,
	})
}
