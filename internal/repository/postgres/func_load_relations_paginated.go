package postgres

import (
	"database/sql"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// RelationSeedPayload structure temporaire pour l'amorçage
type RelationSeedPayload struct {
	CallerID  int64
	TargetID  int64
	State     int
	CreatedAt time.Time
}

// FuncLoadRelationsPaginated appelle la fonction SQL auth.func_load_relations_paginated
func FuncLoadRelationsPaginated(limit, offset int) ([]RelationSeedPayload, error) {
	query := `SELECT * FROM auth.func_load_relations_paginated($1, $2)`
	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	var relations []RelationSeedPayload
	for rows.Next() {
		var r RelationSeedPayload
		if err := rows.Scan(&r.CallerID, &r.TargetID, &r.State, &r.CreatedAt); err == nil {
			relations = append(relations, r)
		}
	}
	return relations, nil
}
