package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

func FuncLoadRecentPosts(ctx context.Context, days int) ([]post_models.PostPayload, error) {
	query := `SELECT * FROM content.func_load_recent_posts($1)`

	rows, err := postgres.PostgresDB.Query(query, days)
	if err != nil {
		numan_log.Error(ctx).Err(err).Msg("Échec de l'exécution de la requête SQL (Query)")
		return nil, numan_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			numan_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des rows Postgres (RecentPosts)")
		}
	}(rows)

	// NOUVEAU
	return scanPosts(ctx, rows)
}
