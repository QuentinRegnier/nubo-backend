package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

// timelineSeedPayload structure temporaire pour la reconstruction L1
type timelineSeedPayload struct {
	PostID    int64
	UserID    int64
	CreatedAt time.Time
}

// FuncLoadTimelineSeedPaginated appelle la fonction SQL content.func_load_timeline_seed_paginated
func FuncLoadTimelineSeedPaginated(ctx context.Context, limit, offset int) ([]timelineSeedPayload, error) {
	query := `SELECT * FROM content.func_load_timeline_seed_paginated($1, $2)`
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

	var seeds []timelineSeedPayload
	for rows.Next() {
		var s timelineSeedPayload
		if err := rows.Scan(&s.PostID, &s.UserID, &s.CreatedAt); err == nil {
			seeds = append(seeds, s)
		}
	}
	return seeds, nil
}
