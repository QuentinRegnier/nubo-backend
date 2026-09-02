package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncLoadUserPosts est le fallback absolu. Il ramène le payload complet depuis L3 pour hydrater la RAM.
func FuncLoadUserPosts(ctx context.Context, userID int64, limit int64, offset int64) ([]post_models.PostPayload, error) {
	query := `SELECT * FROM content.func_load_user_posts($1, $2, $3)`

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

	return scanPosts(rows)
}
