package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// 1. MOTEUR DE RECOMMANDATION ET CLASSEMENTS
// ============================================================================

// UpdatePostRecommendationScore recalcule le score en chargeant l'entité (utilisé par les interactions en temps réel).
func UpdatePostRecommendationScore(ctx context.Context, postID int64, hashtags []string) {
	var p domain.PostRequest
	if err := redis.Posts.GetObject(ctx, postID, &p); err != nil {
		posts, err := GetPostsView([]int64{postID})
		if err != nil || len(posts) == 0 {
			log.Printf("⚠️ Impossible de scorer le post %d : Introuvable", postID)
			return
		}
		p = posts[0]
	}

	if len(hashtags) == 0 && len(p.Hashtags) > 0 {
		hashtags = p.Hashtags
	}

	mediaCount := 0
	if p.HasMedia || len(p.MediaIDs) > 0 {
		mediaCount = 1
	}

	// AJOUT : p.Visibility et 0 (pour isFlagged)
	// TODO rendre dynamique isFlagged
	UpdateScoreWithMetrics(ctx, postID, p.LikeCount, p.CommentCount, mediaCount, p.CreatedAt, hashtags, p.Visibility, 0)
}

// EvaluatePostAfterLike force l'insertion du post avec sa valeur absolue post-sauvegarde BDD.
func EvaluatePostAfterLike(ctx context.Context, postID int64, totalLikes float64, hashtags []string) {
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankLikesStrict, totalLikes, postID, variables.MaxRankElements)
	UpdatePostRecommendationScore(ctx, postID, hashtags)
}

// EvaluatePostAfterView force l'insertion du post avec sa valeur absolue post-sauvegarde BDD.
func EvaluatePostAfterView(ctx context.Context, postID int64, totalViews float64, hashtags []string) {
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankViewsStrict, totalViews, postID, variables.MaxRankElements)
	UpdatePostRecommendationScore(ctx, postID, hashtags)
}

// ============================================================================
// 2. LECTURE ET FALLBACKS (L1 -> L2 -> L3)
// ============================================================================

func GetRankedPosts(ctx context.Context, rankType string, offset int64, limit int64) ([]domain.PostRequest, error) {
	if offset >= variables.MaxRankElements {
		filter := map[string]any{}
		var sort map[string]any

		switch rankType {
		case "likes:strict", "likes:global":
			sort = map[string]any{"like_count": -1, "created_at": -1}
		case "views:strict", "views:global":
			sort = map[string]any{"view_count": -1, "created_at": -1}
		case "global", "recent:global", "recent:strict":
			sort = map[string]any{"created_at": -1}
		default:
			sort = map[string]any{"created_at": -1}
		}

		// L2 (MongoDB)
		docs, err := mongo.Posts.GetPaginated(filter, sort, offset, limit)
		if err != nil {
			// L3 (PostgreSQL)
			return getPostsFromPostgresPaginated(ctx, rankType, offset, limit)
		}

		var posts []domain.PostRequest
		for _, doc := range docs {
			var p domain.PostRequest
			if err := pkg.ToStruct(doc, &p); err == nil {
				posts = append(posts, p)
			}
		}
		return posts, nil
	}

	key := fmt.Sprintf("most_cache:rank:%s", rankType)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}

func GetTagPosts(ctx context.Context, slug string, offset int64, limit int64) ([]domain.PostRequest, error) {
	if offset >= variables.MaxTagElements {
		posts, err := getPostsFromMongoPaginated("hashtags", slug, offset, limit)
		if err != nil {
			// L3 (PostgreSQL)
			query := `SELECT id FROM content.posts WHERE $1 = ANY(hashtags) AND visibility != 2 ORDER BY created_at DESC OFFSET $2 LIMIT $3`
			rows, err := postgres.PostgresDB.QueryContext(ctx, query, slug, offset, limit)
			if err != nil {
				return []domain.PostRequest{}, fmt.Errorf("erreur requête L3 Postgres tag: %w", err)
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

// UpdateScoreWithMetrics est le moteur mathématique et Redis pur (utilisé par le Cron pour éviter les requêtes BDD).
func UpdateScoreWithMetrics(ctx context.Context, postID int64, likes, comments, mediaCount int, createdAt time.Time, hashtags []string, visibility int, isFlagged int) {
	ageSeconds := time.Since(createdAt).Seconds()

	baseOpts := ScoreOptions{
		LikesCount:    likes,
		CommentsCount: comments,
		MediaCount:    mediaCount,
		AgeSeconds:    ageSeconds,
		IsDeleted:     visibility == 0,
		IsReported:    isFlagged == 0,
	}

	optsRecent := baseOpts
	optsRecent.BoostRecent = variables.BoostRecent

	optsLikes := baseOpts
	optsLikes.BoostLikes = variables.BoostLikes

	optsComments := baseOpts
	optsComments.BoostComments = variables.BoostComments

	scoreGlobal := CalculateRecommendationScore(postID, baseOpts)
	scoreBoostRecent := CalculateRecommendationScore(postID, optsRecent)
	scoreBoostLikes := CalculateRecommendationScore(postID, optsLikes)
	scoreBoostComments := CalculateRecommendationScore(postID, optsComments)

	scoreStrictRecent := float64(createdAt.UnixMilli())

	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankGlobal, scoreGlobal, postID, variables.MaxRankElements)
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankRecentGlobal, scoreBoostRecent, postID, variables.MaxRankElements)
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankLikesGlobal, scoreBoostLikes, postID, variables.MaxRankElements)
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankCommentsGlobal, scoreBoostComments, postID, variables.MaxRankElements)
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankRecentStrict, scoreStrictRecent, postID, variables.MaxRankElements)

	if len(hashtags) > 0 {
		officialTags := make(map[string]bool)
		for _, hashtag := range hashtags {
			if slug, found := GetTagFromKeyword(ctx, hashtag); found {
				officialTags[slug] = true
			}
		}

		for slug := range officialTags {
			tagKey := fmt.Sprintf(variables.RedisKeyRankTag, slug)
			_ = redis.ZAddWithCap(ctx, tagKey, scoreGlobal, postID, variables.MaxTagElements)
		}
	}
}
