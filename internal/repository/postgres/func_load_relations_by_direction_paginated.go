package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

func FuncLoadRelationsByDirectionPaginated(ctx context.Context, userID int64, state int, direction string, limit int, offset int) ([]int64, error) {
	query := `SELECT matched_id FROM auth.func_load_relations_paginated_by_direction($1, $2::smallint, $3, $4, $5)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID, state, direction, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des rows Postgres (RecentPosts)")
		}
	}(rows)

	var targetIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			targetIDs = append(targetIDs, id)
		}
	}
	return targetIDs, nil
}
