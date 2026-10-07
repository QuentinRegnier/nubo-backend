package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

func FuncLoadPostIDsByTagPaginated(ctx context.Context, tag string, offset, limit int64) ([]int64, error) {
	query := `SELECT id FROM content.func_load_post_ids_by_tag_paginated($1, $2::integer, $3::integer)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, tag, offset, limit)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Erreur lors de l'itération sur les résultats SQL (rows.Err)")
		return nil, nubo_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			nubo_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
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
