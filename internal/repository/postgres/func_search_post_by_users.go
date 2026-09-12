package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/lib/pq"
)

// FuncSearchPostIDsByUsers appelle la procédure SQL de recherche pour un panel d'utilisateurs avec tri.
func FuncSearchPostIDsByUsers(ctx context.Context, userIDs []int64, orderMode int, offset, limit int64) ([]int64, error) {
	if len(userIDs) == 0 {
		return []int64{}, nil
	}
	query := `SELECT id FROM content.func_search_post_ids_by_users($1, $2::smallint, $3::integer, $4::integer)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, pq.Array(userIDs), orderMode, offset, limit)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres Search Users)")
		}
	}(rows)

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
