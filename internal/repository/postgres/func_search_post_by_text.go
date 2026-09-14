package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncSearchPostIDsByText appelle la recherche Full-Text Postgres.
func FuncSearchPostIDsByText(ctx context.Context, queryStr string, orderMode int, offset, limit int64) ([]int64, error) {
	query := `SELECT id FROM content.func_search_post_ids_by_text($1, $2::integer, $3::integer, $4::integer)`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, queryStr, orderMode, offset, limit)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur fermeture FuncSearchPostIDsByText")
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
