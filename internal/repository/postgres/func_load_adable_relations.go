package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

type relationAddable struct {
	TargetID int64
	State    int
}

// FuncLoadAddableRelations appelle la fonction SQL pure pour récupérer les abonnements et amis.
func FuncLoadAddableRelations(ctx context.Context, callerID int64) ([]relationAddable, error) {
	// Appel strict de la fonction déclarée dans schema.sql
	query := `SELECT target_id, state FROM auth.func_load_addable_relations($1)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, callerID)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			nubo_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var relations []relationAddable
	for rows.Next() {
		var r relationAddable
		if err := rows.Scan(&r.TargetID, &r.State); err == nil {
			relations = append(relations, r)
		}
	}
	return relations, nil
}
