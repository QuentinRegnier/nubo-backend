package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncLoadUserPosts est le fallback absolu. Il ramène le payload complet depuis L3 pour hydrater la RAM.
func FuncLoadUserPosts(ctx context.Context, userID int64, limit int64, offset int64) ([]post_models.PostPayload, error) {
	query := `SELECT id, user_id, content, visibility, like_count, comment_count, view_count, hashtags, mentions, media_ids, created_at, updated_at FROM content.get_user_posts($1, $2, $3)`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(rows)

	return scanPosts(rows)
}
