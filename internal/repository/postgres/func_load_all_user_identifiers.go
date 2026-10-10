package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

type userIdentifiers struct {
	Username *string
	Email    *string
	Phone    *string
}

func FuncLoadAllUserIdentifiers(ctx context.Context) ([]userIdentifiers, error) {
	query := `SELECT username, email, phone FROM auth.func_load_all_user_identifiers()`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, numan_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			numan_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var results []userIdentifiers
	for rows.Next() {
		var idents userIdentifiers
		if err := rows.Scan(&idents.Username, &idents.Email, &idents.Phone); err == nil {
			results = append(results, idents)
		}
	}
	return results, nil
}
