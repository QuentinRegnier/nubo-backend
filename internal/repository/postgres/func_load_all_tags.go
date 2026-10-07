package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncLoadAllTags récupère tous les slugs actifs depuis la base de données.
func FuncLoadAllTags(ctx context.Context) ([]string, error) {
	sqlStatement := `SELECT slug FROM content.func_load_all_tags()`

	rows, err := postgres.PostgresDB.Query(sqlStatement)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Échec de l'exécution de la requête SQL (Query)")
		return nil, nubo_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			nubo_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var tags []string

	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err == nil {
			tags = append(tags, slug)
		} else {
			nubo_log.Error(ctx).Err(err).Msg("Erreur lors du scan d'un tag")
		}
	}

	if err = rows.Err(); err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Erreur lors de l'itération sur les résultats SQL (rows.Err)")
		return nil, nubo_error.NewInternal()
	}

	return tags, nil
}
