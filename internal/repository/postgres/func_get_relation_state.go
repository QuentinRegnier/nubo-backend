package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncGetRelationState interroge directement la fonction SQL compilée pour obtenir l'état.
func FuncGetRelationState(ctx context.Context, callerID int64, targetID int64) (int, error) {
	query := `SELECT auth.func_get_relation_state($1, $2)`

	var state int
	err := postgres.PostgresDB.QueryRowContext(ctx, query, callerID, targetID).Scan(&state)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil // Techniquement impossible car la fonction SQL gère le NOT FOUND, mais on sécurise.
		}
		return 0, fmt.Errorf("erreur postgres FuncGetRelationState: %w", err)
	}

	return state, nil
}
