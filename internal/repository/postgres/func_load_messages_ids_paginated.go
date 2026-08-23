package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
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
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
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
