package postgres

import (
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

func FuncLoadPostsPaginated(limit int, offset int) ([]post_models.PostPayload, error) {
	// Le SQL est maintenant encapsulé et pur
	query := `SELECT * FROM content.func_load_posts_paginated($1, $2)`

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

	// NOUVEAU
	return scanPosts(rows)
}
