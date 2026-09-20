package cache_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// 4. AMORÇAGE (SEEDING)
// ============================================================================

// SeedMostCache lit l'intégralité de Postgres pour populer le L1 (RAM), L2 (Mongo) et le MOST Cache.
func SeedMostCache() error {
	ctx := context.Background()

	// ---------------------------------------------------------
	// PHASE 1 : RESTAURATION DU SYSTÈME DE TAGS
	// ---------------------------------------------------------
	logger.Log.Info().Msg("Restauration des tags communautaires depuis SQL...")

	tagsToSync, err := postgres.FuncLoadAllTags()
	if err != nil {
		logger.Log.Error().Err(err).Msg("Erreur lors du chargement des tags")
	} else if len(tagsToSync) > 0 {
		args := make([]interface{}, len(tagsToSync))
		for i, v := range tagsToSync {
			args[i] = v
		}
		_ = redis.Tags.SAdd(ctx, "active", args...)
	}

	// ---------------------------------------------------------
	// PHASE 2 : HYDRATATION DES POSTS ET CLASSEMENTS (PAR BLOCS)
	// ---------------------------------------------------------
	logger.Log.Info().Msg("Hydratation du MOST Cache depuis SQL (Mode Paginated)...")

	limit := 10000
	offset := 0
	totalProcessed := 0

	for {
		posts, err := postgres.FuncLoadPostsPaginated(limit, offset)
		if err != nil {
			return nubo_error.NewInternal(err)
		}
		if len(posts) == 0 {
			break
		}

		for _, p := range posts {
			UpdatePostRecommendationScore(ctx, p)
			_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictLikes, float64(p.LikeCount), p.ID, variables.MaxStrictElements)
			_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictViews, float64(p.ViewCount), p.ID, variables.MaxStrictElements)
		}

		totalProcessed += len(posts)
		logger.Log.Info().Int("posts_processed", totalProcessed).Msg("Seeding en cours...")
		offset += limit
	}

	// ---------------------------------------------------------
	// PHASE 3 : HYDRATATION INVERSÉE (PRE-WARMING FINAL)
	// ---------------------------------------------------------
	logger.Log.Info().Msg("Lancement de l'hydratation inversée (Pre-warming L1/L2)...")

	winnerIDsMap := make(map[int64]bool)
	keys, _ := redis.Keys(ctx, "most_cache:trend:*")

	for _, key := range keys {
		ids, _ := redis.ZRange(ctx, key, 0, -1)
		for _, idStr := range ids {
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				winnerIDsMap[id] = true
			}
		}
	}

	if len(winnerIDsMap) > 0 {
		var ids []int64
		for id := range winnerIDsMap {
			ids = append(ids, id)
		}

		winners, err := postgres.FuncLoadPosts(ids, len(ids), 0)
		if err == nil {
			for _, p := range winners {
				// L1 : Sanctuarisation immédiate en RAM
				_ = object_cache_service.SetPostInObjectCache(ctx, p)

				// L2 : Délégation pour l'insertion par les workers (BulkWrite)
				_ = redis.EnqueueDB(ctx, p.ID, p.UserID, redis.EntityPost, redis.ActionUpdate, p, redis.TargetMongo)
			}
			logger.Log.Info().Int("count", len(winners)).Msg("Posts d'élite sanctuarisés dans l'Object Cache L1 et en cours d'insertion L2.")
		}
	}

	logger.Log.Info().Msg("Synchronisation MongoDB pour les posts des 30 derniers jours...")
	recentPosts, err := postgres.FuncLoadRecentPosts(30)
	if err == nil {
		for _, p := range recentPosts {
			doc, _ := pkg.ToMap(p)
			if doc != nil {
				_ = mongo.Posts.Set(doc)
			}
		}
		logger.Log.Info().Int("count", len(recentPosts)).Msg("Posts récents synchronisés dans MongoDB.")
	}

	_ = redis.SystemStatus.SetPrimitive(ctx, "maintenance", "off")
	logger.Log.Info().Msg("Mode maintenance désactivé. L'API est opérationnelle.")

	return nil
}

// ============================================================================
// 5. AMORÇAGE DU SPEED CACHE (Utilisateurs & Relations & Communautés)
// ============================================================================

// SeedCommunitySpeedCache charge les communautés publiques (Type 3) dans la barre de recherche
func SeedCommunitySpeedCache(ctx context.Context) error {
	logger.Log.Info().Msg("Amorçage SPEED Cache: Chargement des Communautés (Type 3)...")

	communities, err := postgres.FuncLoadActiveCommunities(ctx)
	if err != nil {
		return err
	}

	count := 0
	for _, c := range communities {
		_ = StoreCommunityLiteInSpeedCache(ctx, c)
		count++
	}

	logger.Log.Info().Int("count", count).Msg("SPEED Cache Communautés chargé.")
	return nil
}

// SeedSpeedCache charge les profils allégés et le graphe social relationnel en RAM
func SeedSpeedCache() error {
	ctx := context.Background()
	limit := 10000

	// --- 1. Utilisateurs ---
	logger.Log.Info().Msg("Amorçage SPEED Cache: Chargement des utilisateurs...")
	offsetUsers := 0
	for {
		users, err := postgres.FuncLoadUsersPaginated(limit, offsetUsers)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Erreur DB lors du chargement des users")
			break
		}
		for _, u := range users {
			_ = StoreUserLiteInSpeedCache(ctx, u)
		}
		offsetUsers += len(users)
		if len(users) < limit {
			break
		}
	}
	logger.Log.Info().Int("count", offsetUsers).Msg("SPEED Cache Users chargés.")

	// --- 2. Relations ---
	logger.Log.Info().Msg("Amorçage SPEED Cache: Chargement des relations...")
	offsetRels := 0
	for {
		relations, err := postgres.FuncLoadRelationsPaginated(limit, offsetRels)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Erreur DB lors du chargement des relations")
			break
		}
		for _, rel := range relations {
			// ✅ NOUVEAU : On passe le Timestamp en millisecondes pour le ZSET
			_ = UpdateRelationState(ctx, rel.TargetID, rel.CallerID, rel.State, domain.TimeToMillis(rel.CreatedAt))
		}
		offsetRels += len(relations)
		if len(relations) < limit {
			break
		}
	}
	logger.Log.Info().Int("count", offsetRels).Msg("SPEED Cache Relations chargées.")

	// --- 3. Communautés (Nouveau) ---
	if err := SeedCommunitySpeedCache(ctx); err != nil {
		logger.Log.Warn().Err(err).Msg("Avertissement lors du seeding des communautés")
	}

	// --- 4. Messagerie (Inbox & Conversations) ---
	if err := SeedMessagingSpeedCache(ctx); err != nil {
		logger.Log.Warn().Err(err).Msg("Avertissement lors du seeding de la messagerie")
	}

	return nil
}

// ============================================================================
// 6. AMORÇAGE DU USER CACHE (Timelines)
// ============================================================================

// SeedUserCache reconstruit les chronologies des profils utilisateurs (ZSETs L1)
func SeedUserCache() error {
	ctx := context.Background()
	limit := 10000
	offset := 0

	logger.Log.Info().Msg("Amorçage USER Cache: Construction des timelines (ZSETs)...")

	for {
		seeds, err := postgres.FuncLoadTimelineSeedPaginated(limit, offset)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Erreur DB lors du chargement des timelines")
			break
		}
		for _, s := range seeds {
			_ = AddPostToUserProfile(ctx, s.UserID, s.PostID, float64(s.CreatedAt.UnixMilli()))
		}
		offset += len(seeds)
		if len(seeds) < limit {
			break
		}
	}

	logger.Log.Info().Int("count", offset).Msg("USER Cache: Timelines reconstruites.")
	return nil
}
