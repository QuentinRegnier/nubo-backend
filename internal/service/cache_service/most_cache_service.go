package cache_service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// 1. MOTEUR DE RECOMMANDATION ET CLASSEMENTS
// ============================================================================

// UpdatePostRecommendationScore recalcule le score à partir de l'entité déjà chargée en RAM.
func UpdatePostRecommendationScore(ctx context.Context, p models.PostRequest) {
	mediaCount := 0
	if p.HasMedia || len(p.MediaIDs) > 0 {
		mediaCount = 1
	}

	// AJOUT : p.Visibility et 0 (pour isFlagged)
	// TODO rendre dynamique isFlagged
	UpdateScoreWithMetrics(ctx, p.ID, p.LikeCount, p.CommentCount, p.ViewCount, mediaCount, p.CreatedAt, p.Hashtags, p.Visibility, 0)
}

// EvaluatePostAfterLike force l'insertion du post_service avec sa valeur absolue dans les classements stricts.
func EvaluatePostAfterLike(ctx context.Context, p models.PostRequest) {
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictLikes, float64(p.LikeCount), p.ID, variables.MaxStrictElements)
	UpdatePostRecommendationScore(ctx, p)
}

// EvaluatePostAfterView force l'insertion du post_service avec sa valeur absolue dans les classements stricts.
func EvaluatePostAfterView(ctx context.Context, p models.PostRequest) {
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictViews, float64(p.ViewCount), p.ID, variables.MaxStrictElements)
	UpdatePostRecommendationScore(ctx, p)
}

// ============================================================================
// 2. LECTURE ET FALLBACKS (L1 -> L2 -> L3)
// ============================================================================

func GetRankedPosts(ctx context.Context, rankType string, offset int64, limit int64) ([]models.PostRequest, error) {
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

		var posts []models.PostRequest
		for _, doc := range docs {
			var p models.PostRequest
			if err := pkg.ToStruct(doc, &p); err == nil {
				posts = append(posts, p)
			}
		}
		return posts, nil
	}

	// NOUVEAU : La clé est générée dynamiquement avec la nomenclature stricte ou algorithmique
	key := fmt.Sprintf("most_cache:%s", rankType)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}
func GetTagPosts(ctx context.Context, slug string, offset int64, limit int64) ([]models.PostRequest, error) {
	if offset >= variables.MaxTagElements {
		posts, err := getPostsFromMongoPaginated("hashtags", slug, offset, limit)
		if err != nil {
			// L3 (PostgreSQL)
			query := `SELECT id FROM content.posts WHERE $1 = ANY(hashtags) AND visibility != 2 ORDER BY created_at DESC OFFSET $2 LIMIT $3`
			rows, err := postgres.PostgresDB.QueryContext(ctx, query, slug, offset, limit)
			if err != nil {
				return []models.PostRequest{}, fmt.Errorf("erreur requête L3 Postgres tag: %w", err)
			}
			defer func(rows *sql.Rows) {
				err := rows.Close()
				if err != nil {
					log.Printf("⚠️ Erreur fermeture rows L3 Postgres tag: %v", err)
				}
			}(rows)

			var ids []int64
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err == nil {
					ids = append(ids, id)
				}
			}
			return GetPostsView(ids)
		}
		return posts, nil
	}

	key := fmt.Sprintf("most_cache:idx:tag:%s", slug)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}

// UpdateTrendZSETs distribue le score de tendance global dans les différents rayons (buckets) Redis.
// C'est le bras armé de la persistance algorithmique (TDD §3.4).
func UpdateTrendZSETs(ctx context.Context, postID int64, score float64, hashtags []string, date, hour, week string) error {
	// 1. Bucket Horaire Global
	hourlyKey := fmt.Sprintf(variables.RedisKeyTrendGlobalHourly, hour)
	if err := redis.ZAddWithCap(ctx, hourlyKey, score, postID, variables.TDDMaxZSET); err != nil {
		return fmt.Errorf("zadd hourly: %w", err)
	}

	// 2. Bucket Journalier Global
	dailyKey := fmt.Sprintf(variables.RedisKeyTrendGlobalDaily, date)
	if err := redis.ZAddWithCap(ctx, dailyKey, score, postID, variables.TDDMaxZSET); err != nil {
		return fmt.Errorf("zadd daily: %w", err)
	}

	// 3. Buckets par Tags Canoniques & Leaderboard
	if len(hashtags) > 0 {
		officialTags := make(map[string]bool)
		for _, hashtag := range hashtags {
			if slug, found := feed_service.GetTagFromKeyword(ctx, hashtag); found {
				officialTags[slug] = true
			}
		}

		for slug := range officialTags {
			// Bucket Journalier du Tag (Injection de slug + date)
			tagDailyKey := fmt.Sprintf(variables.RedisKeyTrendTagDaily, slug, date)
			_ = redis.ZAddWithCap(ctx, tagDailyKey, score, postID, variables.TDDMaxZSET)

			// Bucket Hebdomadaire du Tag (Injection de slug + week)
			tagWeeklyKey := fmt.Sprintf(variables.RedisKeyTrendTagWeekly, slug, week)
			_ = redis.ZAddWithCap(ctx, tagWeeklyKey, score, postID, variables.TDDMaxZSET)

			// Leaderboard Global des Hashtags (Le slug est le membre, le score pousse le hashtag)
			_ = redis.ZAddWithCap(ctx, variables.RedisKeyHashtagLeaderboard, score, slug, variables.TDDMaxZSET)
		}
	}

	return nil
}

// UpdateScoreWithMetrics orchestre le calcul et la distribution des scores.
func UpdateScoreWithMetrics(ctx context.Context, postID int64, likes, comments, views, mediaCount int, createdAt time.Time, hashtags []string, visibility int, isFlagged int) {
	ageSeconds := time.Since(createdAt).Seconds()

	baseOpts := feed_service.ScoreOptions{
		LikesCount:    likes,
		CommentsCount: comments,
		ViewCount:     views,
		MediaCount:    mediaCount,
		AgeSeconds:    ageSeconds,
		IsDeleted:     visibility == 0,
		IsReported:    isFlagged == 0,
	}
	scoreGlobal := feed_service.CalculateRecommendationScore(postID, baseOpts)

	scoreStrictRecent := float64(createdAt.UnixMilli())
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictRecent, scoreStrictRecent, postID, variables.MaxStrictElements)

	now := time.Now().UTC()
	year, isoweek := now.ISOWeek()
	weekStr := fmt.Sprintf("%d-W%02d", year, isoweek) // Ex: "2026-W20"

	// Appel modifié pour transmettre weekStr
	_ = UpdateTrendZSETs(ctx, postID, scoreGlobal, hashtags, now.Format("20060102"), now.Format("2006010215"), weekStr)
}
