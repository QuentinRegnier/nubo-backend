package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
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
	// 1. Récupération des données brutes du post (Indispensable pour l'algorithme)
	var p domain.PostRequest
	if err := redis.Posts.GetObject(ctx, postID, &p); err != nil {
		// Fallback L2/L3 si le post n'est pas dans le cache L1
		posts, err := GetPostsView([]int64{postID})
		if err != nil || len(posts) == 0 {
			log.Printf("⚠️ Impossible de scorer le post %d : Introuvable", postID)
			return
		}
		p = posts[0]
	}

	// Si les hashtags n'ont pas été fournis dans l'événement, on utilise ceux de l'entité
	if len(hashtags) == 0 && len(p.Hashtags) > 0 {
		hashtags = p.Hashtags
	}

	// 2. Construction de la base algorithmique
	ageSeconds := time.Since(p.CreatedAt).Seconds()
	mediaCount := len(p.MediaIDs)

	baseOpts := ScoreOptions{
		LikesCount:    p.LikeCount,
		CommentsCount: p.CommentCount,
		MediaCount:    mediaCount,
		AgeSeconds:    ageSeconds,
		// L'AuthorGrade et AuthorPostsInWindow pourront être hydratés ici plus tard
	}

	// 3. Calcul des différents flux
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

	// Tri chronologique strict : on utilise le timestamp pour figer sa position
	scoreStrictRecent := float64(p.CreatedAt.UnixMilli())

	// 4. Mise à jour des ZSETs globaux (Via Script Lua Atomique)
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankGlobal, scoreGlobal, postID, variables.MaxRankElements)
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankRecentGlobal, scoreBoostRecent, postID, variables.MaxRankElements)
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankLikesGlobal, scoreBoostLikes, postID, variables.MaxRankElements)
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankCommentsGlobal, scoreBoostComments, postID, variables.MaxRankElements)
	_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankRecentStrict, scoreStrictRecent, postID, variables.MaxRankElements)

	// 5. Mise à jour des canaux par Tag
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
// 1.5 VECTORISATION DE CONTENU (PILIER 3)
// ============================================================================

// StoreContentVector calcule et stocke de manière asynchrone le vecteur du post.
func StoreContentVector(ctx context.Context, post domain.PostRequest) {
	// Génération mathématique du vecteur (224 dimensions)
	vector := GenerateContentVector(post.CreatedAt, post.Hashtags, post.UserID)

	// Sérialisation JSON (le tableau de float32 est extrêmement léger)
	vectorBytes, err := json.Marshal(vector)
	if err != nil {
		log.Printf("⚠️ Erreur sérialisation vecteur post %d: %v", post.ID, err)
		return
	}

	// Stockage Redis avec TTL 7 jours (Selon TDD)
	key := fmt.Sprintf(variables.RedisKeyContentVector, post.ID)
	err = redisgo.Rdb.Set(ctx, key, vectorBytes, 7*24*time.Hour).Err()
	if err != nil {
		log.Printf("⚠️ Erreur sauvegarde Redis vecteur post %d: %v", post.ID, err)
	}
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
		_, err := fmt.Sscanf(idStr, "%d", &id)
		if err != nil {
			return nil, err
		}
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
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("⚠️ Erreur fermeture rows L3 Postgres paginé: %v", err)
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
		_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankLikesStrict, likeCount, p.ID, variables.MaxRankElements)
	}

	log.Println("✅ Seeding terminé : synchronisation complète L1, L2 et MOST Cache.")
	return nil
}
