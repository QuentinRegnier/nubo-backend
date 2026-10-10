package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
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
		numan_log.Error(ctx).Err(err).Msg("Erreur SQL inattendue lors de la vérification de l'enregistrement")
		return 0, numan_error.NewInternal()
	}

	return state, nil
}
