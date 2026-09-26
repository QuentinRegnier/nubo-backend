package cache_service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/vmihailenco/msgpack/v5"
)

// ############################################################################
// # MOTEUR DE RECOMMANDATION ET CLASSEMENTS (MOST CACHE)
// ############################################################################

// UpdatePostRecommendationScore recalcule le score à partir de l'entité déjà chargée en RAM.
func UpdatePostRecommendationScore(ctx context.Context, postPayload post_models.PostPayload) {
	mediaCount := 0
	if postPayload.HasMedia || len(postPayload.MediaIDs) > 0 {
		mediaCount = 1
	}

	UpdateScoreWithMetrics(
		ctx,
		postPayload.ID,
		postPayload.LikeCount,
		postPayload.CommentCount,
		postPayload.ViewCount,
		mediaCount,
		domain.MillisToTime(postPayload.CreatedAt),
		postPayload.Hashtags,
		postPayload.IndirectHashtags,
		postPayload.Visibility,
		postPayload.ReportCount,
		postPayload.PriorityLevel,
	)
}

// EvaluatePostAfterLike force l'insertion du post avec sa valeur absolue dans les classements stricts.
func EvaluatePostAfterLike(ctx context.Context, postPayload post_models.PostPayload) {
	errRedis := redis.ZAddWithCap(ctx, variables.RedisKeyStrictLikes, float64(postPayload.LikeCount), postPayload.ID, variables.MaxStrictElements)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Int64("post_id", postPayload.ID).Msg("Impossible de mettre à jour le classement strict des likes")
	}
	UpdatePostRecommendationScore(ctx, postPayload)
}

// EvaluatePostAfterView force l'insertion du post avec sa valeur absolue dans les classements stricts.
func EvaluatePostAfterView(ctx context.Context, postPayload post_models.PostPayload) {
	errRedis := redis.ZAddWithCap(ctx, variables.RedisKeyStrictViews, float64(postPayload.ViewCount), postPayload.ID, variables.MaxStrictElements)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Int64("post_id", postPayload.ID).Msg("Impossible de mettre à jour le classement strict des vues")
	}
	UpdatePostRecommendationScore(ctx, postPayload)
}

// ############################################################################
// # LECTURE ET FALLBACKS (L1 -> L2 -> L3)
// ############################################################################

// GetRankedPosts récupère les classements stricts avec auto-bascule sur Mongo/Postgres si hors RAM.
func GetRankedPosts(ctx context.Context, rankType string, offset int64, limit int64) ([]post_models.PostPayload, error) {

	// Si on dépasse la capacité RAM (MaxStrictElements), on interroge les bases de données froides.
	if offset >= variables.MaxStrictElements {
		mongoFilter := map[string]any{}
		var mongoSort map[string]any

		switch rankType {
		case "strict:likes":
			mongoSort = map[string]any{"like_count": -1, "created_at": -1}
		case "strict:views":
			mongoSort = map[string]any{"view_count": -1, "created_at": -1}
		case "strict:recent":
			mongoSort = map[string]any{"created_at": -1}
		default:
			mongoSort = map[string]any{"created_at": -1} // Fallback sécurisé
		}

		// TENTATIVE L2 (MongoDB)
		mongoDocuments, errMongo := mongo.Posts.GetPaginated(mongoFilter, mongoSort, offset, limit)
		if errMongo != nil {
			// FALLBACK L3 (PostgreSQL)
			return getPostsFromPostgresPaginated(ctx, rankType, offset, limit)
		}

		var hydratedPosts []post_models.PostPayload
		for _, document := range mongoDocuments {
			var postPayload post_models.PostPayload
			if errStruct := pkg.ToStruct(document, &postPayload); errStruct == nil {
				hydratedPosts = append(hydratedPosts, postPayload)
			}
		}
		return hydratedPosts, nil
	}

	// TENTATIVE L1 (RAM)
	return fetchAndHydrateFromCollection(ctx, redis.RankedPosts, rankType, offset, limit)
}

// GetTagPosts récupère la timeline d'un hashtag précis.
func GetTagPosts(ctx context.Context, slug string, offset int64, limit int64) ([]post_models.PostPayload, error) {

	// Si on dépasse la capacité RAM
	if offset >= variables.MaxTagElements {
		postsFromMongo, errMongo := getPostsFromMongoPaginated("hashtags", slug, offset, limit)
		if errMongo != nil {
			// FALLBACK L3 (PostgreSQL) - Pur DDD
			pgIDs, errPg := postgres.FuncLoadPostIDsByTagPaginated(ctx, slug, offset, limit)
			if errPg != nil {
				logger.Log.Error().Err(errPg).Str("slug", slug).Msg("Erreur L3 lors de la pagination des tags")
				return []post_models.PostPayload{}, nubo_error.NewInternal()
			}
			return object_cache_service.GetPostsView(pgIDs)
		}
		return postsFromMongo, nil
	}

	return fetchAndHydrateFromCollection(ctx, redis.TagPosts, slug, offset, limit)
}

// UpdateTrendZSETs distribue le score de tendance global dans les différents rayons Redis.
func UpdateTrendZSETs(ctx context.Context, postID int64, score float64, directTags []string, indirectTags []string, currentDate string, currentHour string, currentWeek string) error {

	// 1. Buckets Globaux
	if err := redis.TrendGlobalHourly.ZAddWithCap(ctx, currentHour, score, postID, variables.TDDMaxZSET); err != nil {
		logger.Log.Error().Err(err).Msg("Impossible de mettre à jour la tendance horaire")
		return nubo_error.NewInternal()
	}

	if err := redis.TrendGlobalDaily.ZAddWithCap(ctx, currentDate, score, postID, variables.TDDMaxZSET); err != nil {
		logger.Log.Error().Err(err).Msg("Impossible de mettre à jour la tendance journalière")
		return nubo_error.NewInternal()
	}

	// 2. Buckets par Tags (Sérialisation Msgpack pour le flag IsIndirect)
	processTags := func(tagsList []string, isIndirect bool) {
		for _, hashtag := range tagsList {
			if cleanSlug, isFound := service.GetTagFromKeyword(ctx, hashtag); isFound {

				tagItem := lite_models.TagPostItem{PostID: postID, IsIndirect: isIndirect}
				binaryData, _ := msgpack.Marshal(tagItem)
				memberBinaryString := string(binaryData)

				_ = redis.TrendTagDaily.ZAddWithCap(ctx, fmt.Sprintf("%s:%s", cleanSlug, currentDate), score, memberBinaryString, variables.TDDMaxZSET)
				_ = redis.TrendTagWeekly.ZAddWithCap(ctx, fmt.Sprintf("%s:%s", cleanSlug, currentWeek), score, memberBinaryString, variables.TDDMaxZSET)

				// Leaderboard ne stocke que le slug (chaîne de caractères)
				_ = redis.HashtagLeaderboard.ZAddWithCap(ctx, "global", score, cleanSlug, variables.TDDMaxZSET)
			}
		}
	}

	processTags(directTags, false)
	processTags(indirectTags, true)

	return nil
}

// UpdateScoreWithMetrics orchestre le calcul mathématique et la distribution des scores.
func UpdateScoreWithMetrics(ctx context.Context, postID int64, likesCount int, commentsCount int, viewsCount int, mediaCount int, createdAt time.Time, directTags []string, indirectTags []string, visibility int, reportCount int, priorityLevel int) {

	ageInSeconds := time.Since(createdAt).Seconds()

	baseOptions := service.ScoreOptions{
		LikesCount:    likesCount,
		CommentsCount: commentsCount,
		ViewCount:     viewsCount,
		MediaCount:    mediaCount,
		ReportCount:   reportCount,
		AgeSeconds:    ageInSeconds,
		IsDeleted:     visibility == 0,
	}

	globalScore := service.CalculateRecommendationScore(postID, baseOptions)

	priorityMultiplier := 1.0
	switch priorityLevel {
	case 1:
		priorityMultiplier = 1.25 // Certifié
	case 2:
		priorityMultiplier = 2.0 // Partenaire
	case 3:
		priorityMultiplier = 3.0 // Modérateur
	case 4:
		priorityMultiplier = 15.0 // Admin
	}
	globalScore *= priorityMultiplier

	scoreStrictRecent := float64(createdAt.UnixMilli())
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictRecent, scoreStrictRecent, postID, variables.MaxStrictElements)

	currentTime := time.Now().UTC()
	year, isoWeek := currentTime.ISOWeek()
	weekString := fmt.Sprintf("%d-W%02d", year, isoWeek)

	_ = UpdateTrendZSETs(ctx, postID, globalScore, directTags, indirectTags, currentTime.Format("20060102"), currentTime.Format("2006010215"), weekString)
}

// GetPostsByTagFromCache lit le ZSET Msgpack du tag, extrait les IDs et déclenche l'hydratation L1 -> L2 -> L3.
func GetPostsByTagFromCache(ctx context.Context, targetTag string, offset int64, limit int64) ([]post_models.PostPayload, error) {

	// Interface temps réel : on vise le trend quotidien
	currentDateString := time.Now().UTC().Format("20060102")

	// PUR DDD : Combinatoire Tag + Date
	compositeID := fmt.Sprintf("%s:%s", targetTag, currentDateString)

	// Lecture de la grappe de binaires MsgPack en O(log N)
	binaryResultsList, errRedis := redis.TrendTagDaily.ZRevRange(ctx, compositeID, offset, offset+limit-1)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Str("tag", targetTag).Msg("Échec de lecture ZSET Tag Daily")
		return nil, nubo_error.NewInternal()
	}

	if len(binaryResultsList) == 0 {
		return []post_models.PostPayload{}, nil
	}

	var extractedPostIDs []int64

	// DÉSÉRIALISATION DE LA MÉTADONNÉE MSGPACK
	for _, binaryString := range binaryResultsList {
		var tagItem lite_models.TagPostItem
		if errUnmarshal := msgpack.Unmarshal([]byte(binaryString), &tagItem); errUnmarshal == nil {
			extractedPostIDs = append(extractedPostIDs, tagItem.PostID)
		}
	}

	if len(extractedPostIDs) == 0 {
		return []post_models.PostPayload{}, nil
	}

	// Déclenchement de la cascade complète (Object Cache -> Mongo -> Postgres)
	return object_cache_service.GetPostsView(extractedPostIDs)
}
