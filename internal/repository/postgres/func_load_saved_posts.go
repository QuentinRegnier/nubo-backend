package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

func FuncLoadSavedPosts(ctx context.Context, userID int64, limit int, offset int) ([]saved_models.SavedPayload, error) {
	query := `SELECT id, user_id, post_id, created_at FROM content.func_load_saved_posts($1, $2, $3)`
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

	var saveds []saved_models.SavedPayload
	for rows.Next() {
		var s saved_models.SavedPayload
		if err := rows.Scan(&s.ID, &s.UserID, &s.PostID, &s.CreatedAt); err == nil {
			saveds = append(saveds, s)
		}
	}
	return saveds, nil
}
