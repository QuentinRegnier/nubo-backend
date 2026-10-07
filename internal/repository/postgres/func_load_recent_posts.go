package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

func FuncLoadRecentPosts(ctx context.Context, days int) ([]post_models.PostPayload, error) {
	query := `SELECT * FROM content.func_load_recent_posts($1)`

	rows, err := postgres.PostgresDB.Query(query, days)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Échec de l'exécution de la requête SQL (Query)")
		return nil, nubo_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			nubo_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des rows Postgres (RecentPosts)")
		}
	}(rows)

	// NOUVEAU
	return scanPosts(ctx, rows)
}
