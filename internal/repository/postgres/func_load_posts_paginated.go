package postgres

import (
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

func FuncLoadPostsPaginated(limit int, offset int) ([]models.PostRequest, error) {
	query := `
		SELECT 
			p.id, p.user_id, p.content, p.hashtags, p.identifiers, p.media_ids, 
			p.visibility, p.location, p.created_at, p.updated_at, p.like_count, 
			p.comment_count, p.view_count, p.has_media, p.vector, p.vector_version
		FROM content.posts p
		WHERE p.visibility != 2
		ORDER BY p.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de FuncLoadPostsPaginated: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur lors de la fermeture des rows dans FuncLoadPostsPaginated:", err)
		}
	}(rows)

	// NOUVEAU
	return scanPosts(rows)
}
