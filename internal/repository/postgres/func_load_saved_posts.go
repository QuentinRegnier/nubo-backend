package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

func FuncLoadSavedPosts(ctx context.Context, userID int64, limit int, offset int) ([]saved_models.SavedPayload, error) {
	query := `SELECT id, user_id, post_id, created_at FROM content.func_load_saved_posts($1, $2, $3)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
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
