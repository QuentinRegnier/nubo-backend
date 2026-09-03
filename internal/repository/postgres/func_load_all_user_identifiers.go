package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

type UserIdentifiers struct {
	Username *string
	Email    *string
	Phone    *string
}

func FuncLoadAllUserIdentifiers(ctx context.Context) ([]UserIdentifiers, error) {
	query := `SELECT username, email, phone FROM auth.func_load_all_user_identifiers()`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var results []UserIdentifiers
	for rows.Next() {
		var idents UserIdentifiers
		if err := rows.Scan(&idents.Username, &idents.Email, &idents.Phone); err == nil {
			results = append(results, idents)
		}
	}
	return results, nil
}
