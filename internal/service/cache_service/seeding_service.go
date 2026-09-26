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

// ############################################################################
// # SERVICE : AMORÇAGE GLOBAL DU SYSTÈME (SEEDING)
// ############################################################################

// SeedMostCache lit l'intégralité de Postgres pour populer le L1 (RAM), L2 (Mongo) et le MOST Cache.
func SeedMostCache() error {
	backgroundCtx := context.Background()

	// ── PHASE 1 : RESTAURATION DU SYSTÈME DE TAGS ───────────────────────────

	logger.Log.Info().Msg("Restauration des tags communautaires depuis le Cold Storage SQL...")

	tagsListFromPg, errPgTags := postgres.FuncLoadAllTags()
	if errPgTags != nil {
		logger.Log.Error().Err(errPgTags).Msg("Échec L3 lors du chargement initial des tags")
	} else if len(tagsListFromPg) > 0 {
		argsForRedis := make([]interface{}, len(tagsListFromPg))
		for index, tagValue := range tagsListFromPg {
			argsForRedis[index] = tagValue
		}
		_ = redis.Tags.SAdd(backgroundCtx, "active", argsForRedis...)
	}

	// ── PHASE 2 : HYDRATATION DES POSTS ET CLASSEMENTS (PAR BLOCS) ──────────

	logger.Log.Info().Msg("Hydratation du MOST Cache depuis SQL (Mode Paginé)...")

	paginationLimit := 10000
	paginationOffset := 0
	totalPostsProcessed := 0

	for {
		postsBatchFromPg, errPgPosts := postgres.FuncLoadPostsPaginated(paginationLimit, paginationOffset)
		if errPgPosts != nil {
			logger.Log.Error().Err(errPgPosts).Msg("Échec L3 lors du seeding paginé des posts")
			return nubo_error.NewInternal()
		}

		if len(postsBatchFromPg) == 0 {
			break
		}

		for _, postPayload := range postsBatchFromPg {
			UpdatePostRecommendationScore(backgroundCtx, postPayload)

			_ = redis.ZAddWithCap(backgroundCtx, variables.RedisKeyStrictLikes, float64(postPayload.LikeCount), postPayload.ID, variables.MaxStrictElements)
			_ = redis.ZAddWithCap(backgroundCtx, variables.RedisKeyStrictViews, float64(postPayload.ViewCount), postPayload.ID, variables.MaxStrictElements)
		}

		totalPostsProcessed += len(postsBatchFromPg)
		logger.Log.Info().Int("posts_processed", totalPostsProcessed).Msg("Seeding en cours...")
		paginationOffset += paginationLimit
	}

	// ── PHASE 3 : HYDRATATION INVERSÉE (PRE-WARMING FINAL) ──────────────────

	logger.Log.Info().Msg("Lancement de l'hydratation inversée (Pre-warming L1/L2 pour l'élite)...")

	winningPostIDsMap := make(map[int64]bool)
	trendKeysList, _ := redis.Keys(backgroundCtx, "most_cache:trend:*")

	for _, trendKey := range trendKeysList {
		idStringsInTrend, _ := redis.ZRange(backgroundCtx, trendKey, 0, -1)
		for _, idString := range idStringsInTrend {
			if parsedID, errParse := strconv.ParseInt(idString, 10, 64); errParse == nil {
				winningPostIDsMap[parsedID] = true
			}
		}
	}

	if len(winningPostIDsMap) > 0 {
		var winningIDsList []int64
		for id := range winningPostIDsMap {
			winningIDsList = append(winningIDsList, id)
		}

		elitePostsList, errPgElite := postgres.FuncLoadPosts(winningIDsList, len(winningIDsList), 0)
		if errPgElite == nil {
			for _, elitePost := range elitePostsList {
				// L1 : Sanctuarisation immédiate en RAM
				_ = object_cache_service.SetPostInObjectCache(backgroundCtx, elitePost)

				// L2 : Délégation pour l'insertion par les workers (BulkWrite Mongo)
				_ = redis.EnqueueDB(backgroundCtx, elitePost.ID, elitePost.UserID, redis.EntityPost, redis.ActionUpdate, elitePost, redis.TargetMongo)
			}
			logger.Log.Info().Int("count", len(elitePostsList)).Msg("Posts d'élite sanctuarisés dans l'Object Cache L1 et en cours d'insertion L2.")
		}
	}

	logger.Log.Info().Msg("Synchronisation MongoDB pour les posts des 30 derniers jours...")
	recentPostsFromPg, errPgRecent := postgres.FuncLoadRecentPosts(30)

	if errPgRecent == nil {
		for _, recentPost := range recentPostsFromPg {
			documentMap, _ := pkg.ToMap(recentPost)
			if documentMap != nil {
				_ = mongo.Posts.Set(documentMap)
			}
		}
		logger.Log.Info().Int("count", len(recentPostsFromPg)).Msg("Posts récents synchronisés dans le Warm Storage MongoDB.")
	}

	// Déverrouillage de l'API
	_ = redis.SystemStatus.SetPrimitive(backgroundCtx, "maintenance", "off")
	logger.Log.Info().Msg("Mode maintenance désactivé. L'API est opérationnelle.")

	return nil
}

// ############################################################################
// # AMORÇAGE DU SPEED CACHE (Recherche & Relations & Inbox)
// ############################################################################

// SeedCommunitySpeedCache charge les communautés publiques dans la barre de recherche.
func SeedCommunitySpeedCache(ctx context.Context) error {
	logger.Log.Info().Msg("Amorçage SPEED Cache: Chargement des Communautés Publiques...")

	activeCommunitiesFromPg, errPg := postgres.FuncLoadActiveCommunities(ctx)
	if errPg != nil {
		return nubo_error.NewInternal()
	}

	totalLoaded := 0
	for _, communityPayload := range activeCommunitiesFromPg {
		_ = StoreCommunityLiteInSpeedCache(ctx, communityPayload)
		totalLoaded++
	}

	logger.Log.Info().Int("count", totalLoaded).Msg("SPEED Cache Communautés chargé.")
	return nil
}

// SeedSpeedCache charge les profils allégés et le graphe social relationnel en RAM L1.
func SeedSpeedCache() error {
	backgroundCtx := context.Background()
	paginationLimit := 10000

	// ── 1. UTILISATEURS (COMPTES LITE) ──────────────────────────────────────

	logger.Log.Info().Msg("Amorçage SPEED Cache: Chargement des Utilisateurs...")
	offsetUsers := 0
	for {
		usersBatchFromPg, errPg := postgres.FuncLoadUsersPaginated(paginationLimit, offsetUsers)
		if errPg != nil {
			logger.Log.Warn().Err(errPg).Msg("Erreur L3 lors du chargement paginé des utilisateurs")
			break
		}

		for _, userPayload := range usersBatchFromPg {
			_ = StoreUserLiteInSpeedCache(backgroundCtx, userPayload)
		}

		offsetUsers += len(usersBatchFromPg)
		if len(usersBatchFromPg) < paginationLimit {
			break
		}
	}
	logger.Log.Info().Int("count", offsetUsers).Msg("SPEED Cache Users chargé.")

	// ── 2. GRAPHE SOCIAL (RELATIONS) ────────────────────────────────────────

	logger.Log.Info().Msg("Amorçage SPEED Cache: Chargement des Relations...")
	offsetRelations := 0
	for {
		relationsBatchFromPg, errPg := postgres.FuncLoadRelationsPaginated(paginationLimit, offsetRelations)
		if errPg != nil {
			logger.Log.Warn().Err(errPg).Msg("Erreur L3 lors du chargement paginé des relations")
			break
		}

		for _, relationRecord := range relationsBatchFromPg {
			_ = UpdateRelationState(backgroundCtx, relationRecord.TargetID, relationRecord.CallerID, relationRecord.State, domain.TimeToMillis(relationRecord.CreatedAt))
		}

		offsetRelations += len(relationsBatchFromPg)
		if len(relationsBatchFromPg) < paginationLimit {
			break
		}
	}
	logger.Log.Info().Int("count", offsetRelations).Msg("SPEED Cache Relations chargé.")

	// ── 3. COMMUNAUTÉS ──────────────────────────────────────────────────────

	if errComm := SeedCommunitySpeedCache(backgroundCtx); errComm != nil {
		logger.Log.Warn().Err(errComm).Msg("Avertissement lors du seeding des communautés")
	}

	// ── 4. MESSAGERIE (INBOX & CHATS) ───────────────────────────────────────

	if errMsg := SeedMessagingSpeedCache(backgroundCtx); errMsg != nil {
		logger.Log.Warn().Err(errMsg).Msg("Avertissement lors du seeding de la messagerie")
	}

	return nil
}

// ############################################################################
// # AMORÇAGE DU USER CACHE (TIMELINES)
// ############################################################################

// SeedUserCache reconstruit les chronologies des profils utilisateurs (ZSETs L1).
func SeedUserCache() error {
	backgroundCtx := context.Background()
	paginationLimit := 10000
	paginationOffset := 0

	logger.Log.Info().Msg("Amorçage USER Cache: Construction des Timelines L1 (ZSETs)...")

	for {
		timelineSeedsFromPg, errPg := postgres.FuncLoadTimelineSeedPaginated(paginationLimit, paginationOffset)
		if errPg != nil {
			logger.Log.Warn().Err(errPg).Msg("Erreur L3 lors du chargement des graines de timelines")
			break
		}

		for _, timelineRecord := range timelineSeedsFromPg {
			_ = AddPostToUserProfile(backgroundCtx, timelineRecord.UserID, timelineRecord.PostID, float64(timelineRecord.CreatedAt.UnixMilli()))
		}

		paginationOffset += len(timelineSeedsFromPg)
		if len(timelineSeedsFromPg) < paginationLimit {
			break
		}
	}

	logger.Log.Info().Int("count", paginationOffset).Msg("USER Cache: Timelines reconstruites avec succès.")
	return nil
}
