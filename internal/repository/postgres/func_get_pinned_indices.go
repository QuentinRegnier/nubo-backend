package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncGetPinnedIndices récupère les indices d'épingles (Source of Truth)
func FuncGetPinnedIndices(ctx context.Context, userID int64) ([]int, error) {
	query := `SELECT (settings->>'pinned')::int FROM messaging.members WHERE user_id = $1 AND (settings->>'pinned')::int >= 0`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var indices []int
	for rows.Next() {
		var idx int
		if err := rows.Scan(&idx); err == nil {
			indices = append(indices, idx)
		}
	}
	return indices, nil
}
