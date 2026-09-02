package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncLoadMessageIDsPaginated récupère uniquement l'Index depuis le B-Tree Postgres (0.1ms) en appliquant le gel de l'historique.
func FuncLoadMessageIDsPaginated(ctx context.Context, convID int64, offsetID int64, limit int64, direction string, frozenID int64) ([]int64, error) {
	var pFrozenID any = frozenID
	if frozenID == 0 {
		pFrozenID = nil
	}

	query := `SELECT id FROM messaging.func_load_message_ids_paginated($1, $2, $3, $4, $5)`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, convID, offsetID, limit, direction, pFrozenID)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
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
