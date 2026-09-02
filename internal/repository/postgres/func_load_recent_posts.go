package postgres

import (
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

func FuncLoadRecentPosts(days int) ([]post_models.PostPayload, error) {
	query := `SELECT * FROM content.func_load_recent_posts($1)`

	rows, err := postgres.PostgresDB.Query(query, days)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des rows Postgres (RecentPosts)")
		}
	}(rows)

	// NOUVEAU
	return scanPosts(rows)
}
