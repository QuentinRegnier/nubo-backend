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
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/vmihailenco/msgpack/v5"
)

// ============================================================================
// 1. MOTEUR DE RECOMMANDATION ET CLASSEMENTS
// ============================================================================

// UpdatePostRecommendationScore recalcule le score à partir de l'entité déjà chargée en RAM.
func UpdatePostRecommendationScore(ctx context.Context, p post_models.PostPayload) {
	mediaCount := 0
	if p.HasMedia || len(p.MediaIDs) > 0 {
		mediaCount = 1
	}

	// ✅ Transmission séparée des Hashtags Directs et Indirects
	UpdateScoreWithMetrics(ctx, p.ID, p.LikeCount, p.CommentCount, p.ViewCount, mediaCount, domain.MillisToTime(p.CreatedAt), p.Hashtags, p.IndirectHashtags, p.Visibility, p.ReportCount, p.PriorityLevel)
}

// EvaluatePostAfterLike force l'insertion du post_service avec sa valeur absolue dans les classements stricts.
func EvaluatePostAfterLike(ctx context.Context, p post_models.PostPayload) {
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictLikes, float64(p.LikeCount), p.ID, variables.MaxStrictElements)
	UpdatePostRecommendationScore(ctx, p)
}

// EvaluatePostAfterView force l'insertion du post_service avec sa valeur absolue dans les classements stricts.
func EvaluatePostAfterView(ctx context.Context, p post_models.PostPayload) {
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictViews, float64(p.ViewCount), p.ID, variables.MaxStrictElements)
	UpdatePostRecommendationScore(ctx, p)
}

// ============================================================================
// 2. LECTURE ET FALLBACKS (L1 -> L2 -> L3)
// ============================================================================

func GetRankedPosts(ctx context.Context, rankType string, offset int64, limit int64) ([]post_models.PostPayload, error) {
	// Note: On utilise MaxStrictElements comme seuil de sécurité global pour le fallback
	if offset >= variables.MaxStrictElements {
		filter := map[string]any{}
		var sort map[string]any

		switch rankType {
		case "strict:likes":
			sort = map[string]any{"like_count": -1, "created_at": -1}
		case "strict:views":
			sort = map[string]any{"view_count": -1, "created_at": -1}
		case "strict:recent":
			sort = map[string]any{"created_at": -1}
		default:
			sort = map[string]any{"created_at": -1} // Fallback par défaut pour les trends si nécessaire
		}

		// L2 (MongoDB)
		docs, err := mongo.Posts.GetPaginated(filter, sort, offset, limit)
		if err != nil {
			// L3 (PostgreSQL)
			return getPostsFromPostgresPaginated(ctx, rankType, offset, limit)
		}

		var posts []post_models.PostPayload
		for _, doc := range docs {
			var p post_models.PostPayload
			if err := pkg.ToStruct(doc, &p); err == nil {
				posts = append(posts, p)
			}
		}
		return posts, nil
	}

	// NOUVEAU : La clé est générée dynamiquement avec la nomenclature stricte ou algorithmique
	return fetchAndHydrateFromCollection(ctx, redis.RankedPosts, rankType, offset, limit)
}
func GetTagPosts(ctx context.Context, slug string, offset int64, limit int64) ([]post_models.PostPayload, error) {
	if offset >= variables.MaxTagElements {
		posts, err := getPostsFromMongoPaginated("hashtags", slug, offset, limit)
		if err != nil {
			// L3 (PostgreSQL) - Pur DDD
			ids, errPg := postgres.FuncLoadPostIDsByTagPaginated(ctx, slug, offset, limit)
			if errPg != nil {
				return []post_models.PostPayload{}, nubo_error.NewInternal(errPg)
			}
			return object_cache_service.GetPostsView(ids)
		}
		return posts, nil
	}

	return fetchAndHydrateFromCollection(ctx, redis.TagPosts, slug, offset, limit)
}

// UpdateTrendZSETs distribue le score de tendance global dans les différents rayons (buckets) Redis.
func UpdateTrendZSETs(ctx context.Context, postID int64, score float64, directTags []string, indirectTags []string, date, hour, week string) error {
	// 1. Buckets Globaux (Seul le PostID est stocké, soumis au TDDMaxZSET)
	if err := redis.TrendGlobalHourly.ZAddWithCap(ctx, hour, score, postID, variables.TDDMaxZSET); err != nil {
		return nubo_error.NewInternal(err)
	}

	if err := redis.TrendGlobalDaily.ZAddWithCap(ctx, date, score, postID, variables.TDDMaxZSET); err != nil {
		return nubo_error.NewInternal(err)
	}

	// 2. Buckets par Tags (Sérialisation Msgpack pour le contexte IsIndirect)
	processTags := func(tags []string, isIndirect bool) {
		for _, hashtag := range tags {
			if slug, found := service.GetTagFromKeyword(ctx, hashtag); found {
				// ✅ SÉRIALISATION MSGPACK
				item := lite_models.TagPostItem{PostID: postID, IsIndirect: isIndirect}
				b, _ := msgpack.Marshal(item)
				memberBinary := string(b)

				// Respect strict de TDDMaxZSET
				_ = redis.TrendTagDaily.ZAddWithCap(ctx, fmt.Sprintf("%s:%s", slug, date), score, memberBinary, variables.TDDMaxZSET)
				_ = redis.TrendTagWeekly.ZAddWithCap(ctx, fmt.Sprintf("%s:%s", slug, week), score, memberBinary, variables.TDDMaxZSET)

				// Le leaderboard ne stocke que des chaînes de caractères (le slug)
				_ = redis.HashtagLeaderboard.ZAddWithCap(ctx, "global", score, slug, variables.TDDMaxZSET)
			}
		}
	}

	processTags(directTags, false)
	processTags(indirectTags, true)

	return nil
}

// UpdateScoreWithMetrics orchestre le calcul et la distribution des scores.
func UpdateScoreWithMetrics(ctx context.Context, postID int64, likes, comments, views, mediaCount int, createdAt time.Time, directTags []string, indirectTags []string, visibility int, reportCount int, priorityLevel int) {
	ageSeconds := time.Since(createdAt).Seconds()

	baseOpts := service.ScoreOptions{
		LikesCount:    likes,
		CommentsCount: comments,
		ViewCount:     views,
		MediaCount:    mediaCount,
		ReportCount:   reportCount,
		AgeSeconds:    ageSeconds,
		IsDeleted:     visibility == 0,
	}
	scoreGlobal := service.CalculateRecommendationScore(postID, baseOpts)

	multiplier := 1.0
	switch priorityLevel {
	case 1:
		multiplier = 1.25 // Certifié
	case 2:
		multiplier = 2.0 // Partenaire
	case 3:
		multiplier = 3.0 // Modérateur
	case 4:
		multiplier = 15.0 // Admin
	}
	scoreGlobal *= multiplier

	scoreStrictRecent := float64(createdAt.UnixMilli())
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictRecent, scoreStrictRecent, postID, variables.MaxStrictElements)

	now := time.Now().UTC()
	year, isoweek := now.ISOWeek()
	weekStr := fmt.Sprintf("%d-W%02d", year, isoweek)

	// ✅ Appel modifié pour inclure les tags indirects
	_ = UpdateTrendZSETs(ctx, postID, scoreGlobal, directTags, indirectTags, now.Format("20060102"), now.Format("2006010215"), weekStr)
}

// GetPostsByTagFromCache lit le ZSET Msgpack du tag, extrait les IDs, et déclenche l'hydratation L1 -> L2 -> L3.
func GetPostsByTagFromCache(ctx context.Context, tag string, offset int64, limit int64) ([]post_models.PostPayload, error) {
	// Pour l'interface temps réel, on vise le trend quotidien
	dateStr := time.Now().UTC().Format("20060102")

	// ✅ PUR DDD : On délègue la formation de la clé physique à l'abstraction de la Collection.
	// L'identifiant métier est simplement la combinaison du tag et de la date.
	compositeID := fmt.Sprintf("%s:%s", tag, dateStr)

	// Lecture de la grappe de binaires MsgPack en O(log(N))
	binaries, err := redis.TrendTagDaily.ZRevRange(ctx, compositeID, offset, offset+limit-1)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}

	if len(binaries) == 0 {
		return []post_models.PostPayload{}, nil
	}

	var postIDs []int64

	// DÉSÉRIALISATION DE LA MÉTADONNÉE
	for _, binStr := range binaries {
		var item lite_models.TagPostItem
		if err := msgpack.Unmarshal([]byte(binStr), &item); err == nil {
			postIDs = append(postIDs, item.PostID)
		}
	}

	if len(postIDs) == 0 {
		return []post_models.PostPayload{}, nil
	}

	// Déclenchement de la cascade complète (Object Cache -> Mongo -> Postgres)
	return object_cache_service.GetPostsView(postIDs)
}
