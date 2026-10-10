package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

// FuncLoadUserPosts est le fallback absolu. Il ramène le payload complet depuis L3 pour hydrater la RAM.
func FuncLoadUserPosts(ctx context.Context, userID int64, limit int64, offset int64) ([]post_models.PostPayload, error) {
	query := `SELECT * FROM content.func_load_user_posts($1, $2, $3)`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		numan_log.Error(ctx).Err(err).Msg("Échec de l'exécution de la requête SQL (QueryContext)")
		return nil, numan_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			numan_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	return scanPosts(ctx, rows)
}
