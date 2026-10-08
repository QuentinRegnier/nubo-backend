package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// relationSeedPayload structure temporaire pour l'amorçage
type relationSeedPayload struct {
	CallerID  int64
	TargetID  int64
	State     int
	CreatedAt time.Time
}

// FuncLoadRelationsPaginated appelle la fonction SQL auth.func_load_relations_paginated
func FuncLoadRelationsPaginated(ctx context.Context, limit, offset int) ([]relationSeedPayload, error) {
	query := `SELECT * FROM auth.func_load_relations_paginated($1, $2)`
	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Échec de l'exécution de la requête SQL (Query)")
		return nil, nubo_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			nubo_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	var relations []relationSeedPayload
	for rows.Next() {
		var r relationSeedPayload
		if err := rows.Scan(&r.CallerID, &r.TargetID, &r.State, &r.CreatedAt); err == nil {
			relations = append(relations, r)
		}
	}
	return relations, nil
}
