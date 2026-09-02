package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncGetRelationState interroge directement la fonction SQL compilée pour obtenir l'état.
func FuncGetRelationState(ctx context.Context, callerID int64, targetID int64) (int, error) {
	query := `SELECT auth.func_get_relation_state($1, $2)`

	var state int
	err := postgres.PostgresDB.QueryRowContext(ctx, query, callerID, targetID).Scan(&state)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, nubo_error.NewInternal(err)
	}

	return state, nil
}
