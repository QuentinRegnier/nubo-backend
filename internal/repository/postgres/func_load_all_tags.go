package postgres

import (
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncLoadAllTags récupère tous les slugs actifs depuis la base de données.
func FuncLoadAllTags() ([]string, error) {
	sqlStatement := `SELECT slug FROM content.func_load_all_tags()`

	rows, err := postgres.PostgresDB.Query(sqlStatement)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var tags []string

	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err == nil {
			tags = append(tags, slug)
		} else {
			logger.Log.Error().Err(err).Msg("Erreur lors du scan d'un tag")
		}
	}

	if err = rows.Err(); err != nil {
		return nil, nubo_error.NewInternal(err)
	}

	return tags, nil
}
