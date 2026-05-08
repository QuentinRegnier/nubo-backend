package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ============================================================================
// 3. ROUTINES PRIVÉES D'ACCÈS AUX DONNÉES
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
