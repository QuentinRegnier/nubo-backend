package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

func FuncLoadPostsPaginated(ctx context.Context, limit int, offset int) ([]post_models.PostPayload, error) {
	// Le SQL est maintenant encapsulé et pur
	query := `SELECT * FROM content.func_load_posts_paginated($1, $2)`

	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Échec de l'exécution de la requête SQL (Query)")
		return nil, nubo_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	// NOUVEAU
	return scanPosts(ctx, rows)
}
