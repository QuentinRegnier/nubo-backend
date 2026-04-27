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
	"github.com/lib/pq"
)

// ============================================================================
// 1. MOTEUR DE RECOMMANDATION ET CLASSEMENTS
// ============================================================================

// UpdatePostRecommendationScore recalcule le score et actualise les canaux globaux et tags.
func UpdatePostRecommendationScore(ctx context.Context, postID int64, hashtags []string) {
	// TODO: Intégrer les paramètres de personnalisation utilisateur (Feed Cache) plus tard

	scoreGlobal := CalculateRecommendationScore(postID, ScoreOptions{})
	scoreBoostRecent := CalculateRecommendationScore(postID, ScoreOptions{BoostRecent: variables.BoostRecent})
	scoreBoostLikes := CalculateRecommendationScore(postID, ScoreOptions{BoostLikes: variables.BoostLikes})
	scoreBoostViews := CalculateRecommendationScore(postID, ScoreOptions{BoostViews: variables.BoostViews})

	scoreStrictRecent := float64(time.Now().UnixMilli())

	// Pré-hydratation conditionnelle : Si le post remonte via une interaction (sans hashtags fournis), on sécurise le L1
	if len(hashtags) == 0 {
		var tempPost domain.PostRequest
		if err := redis.Posts.GetObject(ctx, postID, &tempPost); err != nil {
			_, _ = GetPostsView([]int64{postID})
		}
	}

	// Mise à jour des canaux globaux
	_ = redis.ZAdd(ctx, "rank:global", scoreGlobal, postID)
	_ = redis.ZRemRangeByRank(ctx, "rank:global", 0, -(variables.MaxRankElements + 1))

	_ = redis.ZAdd(ctx, "rank:recent:global", scoreBoostRecent, postID)
	_ = redis.ZRemRangeByRank(ctx, "rank:recent:global", 0, -(variables.MaxRankElements + 1))

	_ = redis.ZAdd(ctx, "rank:likes:global", scoreBoostLikes, postID)
	_ = redis.ZRemRangeByRank(ctx, "rank:likes:global", 0, -(variables.MaxRankElements + 1))

	_ = redis.ZAdd(ctx, "rank:views:global", scoreBoostViews, postID)
	_ = redis.ZRemRangeByRank(ctx, "rank:views:global", 0, -(variables.MaxRankElements + 1))

	_ = redis.ZAdd(ctx, "rank:recent:strict", scoreStrictRecent, postID)
	_ = redis.ZRemRangeByRank(ctx, "rank:recent:strict", 0, -(variables.MaxRankElements + 1))

	// Mise à jour des canaux par Tag
	if len(hashtags) > 0 {
		officialTags := make(map[string]bool)
		for _, hashtag := range hashtags {
			if slug, found := GetTagFromKeyword(hashtag); found {
				officialTags[slug] = true
			}
		}

		for slug := range officialTags {
			tagKey := fmt.Sprintf("idx:tag:%s", slug)
			_ = redis.ZAdd(ctx, tagKey, scoreGlobal, postID)
			_ = redis.ZRemRangeByRank(ctx, tagKey, 0, -(variables.MaxTagElements + 1))
		}
	}
}

// EvaluatePostAfterLike force l'insertion du post avec sa valeur absolue post-sauvegarde BDD.
func EvaluatePostAfterLike(ctx context.Context, postID int64, totalLikes float64, hashtags []string) {
	strictKey := "rank:likes:strict"
	_ = redis.ZAdd(ctx, strictKey, totalLikes, postID)
	_ = redis.ZRemRangeByRank(ctx, strictKey, 0, -(variables.MaxRankElements + 1))

	UpdatePostRecommendationScore(ctx, postID, hashtags)
}

// EvaluatePostAfterView force l'insertion du post avec sa valeur absolue post-sauvegarde BDD.
func EvaluatePostAfterView(ctx context.Context, postID int64, totalViews float64, hashtags []string) {
	strictKey := "rank:views:strict"
	_ = redis.ZAdd(ctx, strictKey, totalViews, postID)
	_ = redis.ZRemRangeByRank(ctx, strictKey, 0, -(variables.MaxRankElements + 1))

	UpdatePostRecommendationScore(ctx, postID, hashtags)
}

// ============================================================================
// 2. PROFILS UTILISATEURS
// ============================================================================

// AddPostToUserProfile ajoute un post à la grille chronologique d'un profil.
func AddPostToUserProfile(ctx context.Context, userID int64, postID int64) {
	key := fmt.Sprintf("user:posts:%d", userID)
	score := float64(time.Now().UnixMilli())

	if err := redis.ZAdd(ctx, key, score, postID); err != nil {
		return
	}
	_ = redis.ZRemRangeByRank(ctx, key, 0, -(variables.MaxUserPostsElements + 1))
}

// ============================================================================
// 3. LECTURE ET FALLBACKS (L1 -> L2 -> L3)
// ============================================================================

func GetUserProfilePosts(ctx context.Context, userID int64, offset int64, limit int64) ([]domain.PostRequest, error) {
	if offset >= variables.MaxUserPostsElements {
		return getPostsFromMongoPaginated("user_id", userID, offset, limit)
	}

	key := fmt.Sprintf("user:posts:%d", userID)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}

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

	key := fmt.Sprintf("rank:%s", rankType)
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
			defer rows.Close()

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

	key := fmt.Sprintf("idx:tag:%s", slug)
	return fetchAndHydrateFromZSET(ctx, key, offset, limit)
}

// ============================================================================
// 4. ROUTINES PRIVÉES D'ACCÈS AUX DONNÉES
// ============================================================================

func fetchAndHydrateFromZSET(ctx context.Context, key string, offset int64, limit int64) ([]domain.PostRequest, error) {
	idStrings, err := redis.ZRevRange(ctx, key, offset, offset+limit-1)
	if err != nil {
		return nil, fmt.Errorf("erreur lecture ZSET %s: %w", key, err)
	}

	if len(idStrings) == 0 {
		return []domain.PostRequest{}, nil
	}

	var ids []int64
	for _, idStr := range idStrings {
		var id int64
		fmt.Sscanf(idStr, "%d", &id)
		ids = append(ids, id)
	}

	return GetPostsView(ids)
}

func getPostsFromMongoPaginated(field string, value any, offset int64, limit int64) ([]domain.PostRequest, error) {
	filter := map[string]any{field: value}
	sort := map[string]any{"created_at": -1}

	docs, err := mongo.Posts.GetPaginated(filter, sort, offset, limit)
	if err != nil {
		return []domain.PostRequest{}, err
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

func getPostsFromPostgresPaginated(ctx context.Context, rankType string, offset int64, limit int64) ([]domain.PostRequest, error) {
	// TODO: Optimiser ces requêtes avec des vues matérialisées si la BDD dépasse 1M de lignes
	var query string

	switch rankType {
	case "likes:strict":
		query = `
			SELECT p.id FROM content.posts p 
			WHERE p.visibility != 2 
			ORDER BY (SELECT COUNT(*) FROM content.likes l WHERE l.target_id = p.id AND l.target_type = 0) DESC, p.created_at DESC 
			OFFSET $1 LIMIT $2`
	case "views:strict":
		query = `
			SELECT p.id FROM content.posts p 
			WHERE p.visibility != 2 
			ORDER BY (SELECT COUNT(*) FROM content.views v WHERE v.target_id = p.id AND v.target_type = 0) DESC, p.created_at DESC 
			OFFSET $1 LIMIT $2`
	default:
		query = `SELECT id FROM content.posts WHERE visibility != 2 ORDER BY created_at DESC OFFSET $1 LIMIT $2`
	}

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, offset, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}

	return GetPostsView(ids)
}

// ============================================================================
// 5. AMORÇAGE (SEEDING)
// ============================================================================

// SeedMostCache lit l'intégralité de Postgres pour populer le L1 (RAM), L2 (Mongo) et le MOST Cache.
func SeedMostCache() error {
	ctx := context.Background()

	// TODO: Traiter le seeding par lots (Batch) pour éviter une surcharge RAM lors du scan complet de la base
	query := `
		SELECT 
			id, user_id, content, hashtags, identifiers, media_ids, visibility, location, created_at, updated_at,
			(SELECT COUNT(*) FROM content.likes l WHERE l.target_id = p.id AND l.target_type = 0) AS like_count
		FROM content.posts p
		WHERE visibility != 2
	`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("erreur requête seeding: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p domain.PostRequest
		var location sql.NullString
		var likeCount float64

		err := rows.Scan(
			&p.ID, &p.UserID, &p.Content, pq.Array(&p.Hashtags), pq.Array(&p.Identifiers),
			pq.Array(&p.MediaIDs), &p.Visibility, &location, &p.CreatedAt, &p.UpdatedAt,
			&likeCount,
		)
		if err != nil {
			continue
		}

		if location.Valid {
			p.Location = location.String
		}

		// A. Hydratation L2 (MongoDB)
		doc, _ := pkg.ToMap(p)
		if doc != nil {
			_ = mongo.Posts.Set(doc)
		}

		// B. Hydratation L1 (OBJECT Cache)
		_ = redis.Posts.SetObject(ctx, p.ID, p)

		// C. Hydratation MOST Cache (ZSETs algorithmiques)
		UpdatePostRecommendationScore(ctx, p.ID, p.Hashtags)

		// D. Classement STRICT (Absolu)
		_ = redis.ZAdd(ctx, "rank:likes:strict", likeCount, p.ID)
		_ = redis.ZRemRangeByRank(ctx, "rank:likes:strict", 0, -(variables.MaxRankElements + 1))
	}

	log.Println("✅ Seeding terminé : synchronisation complète L1, L2 et MOST Cache.")
	return nil
}
