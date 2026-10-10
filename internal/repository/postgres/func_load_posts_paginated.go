package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

func FuncLoadPostsPaginated(ctx context.Context, limit int, offset int) ([]post_models.PostPayload, error) {
	// Le SQL est maintenant encapsulé et pur
	query := `SELECT * FROM content.func_load_posts_paginated($1, $2)`

	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		numan_log.Error(ctx).Err(err).Msg("Échec de l'exécution de la requête SQL (Query)")
		return nil, numan_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			numan_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	// NOUVEAU
	return scanPosts(ctx, rows)
}
