package postgres

import (
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

func FuncLoadRecentPosts(days int) ([]post_models.PostPayload, error) {
	query := `
		SELECT 
			p.id, p.user_id, p.content, p.hashtags, p.identifiers, p.media_ids, 
			p.visibility, p.location, p.created_at, p.updated_at, p.like_count, 
			p.comment_count, p.view_count, p.has_media, p.vector, p.vector_version
		FROM content.posts p
		WHERE p.visibility != 2
		AND p.created_at >= NOW() - ($1 || ' days')::interval
		ORDER BY p.created_at DESC
	`

	rows, err := postgres.PostgresDB.Query(query, days)
	if err != nil {
		return nil, fmt.Errorf("erreur FuncLoadRecentPosts: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur lors de la fermeture des rows dans FuncLoadRecentPosts:", err)
		}
	}(rows)

	// NOUVEAU
	return scanPosts(rows)
}
