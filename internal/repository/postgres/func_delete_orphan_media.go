package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

type orphanMediaResult struct {
	ID          int64
	StoragePath string
}

// FuncDeleteOrphanMedia exécute la purge SQL et retourne les médias détruits.
func FuncDeleteOrphanMedia(ctx context.Context) ([]orphanMediaResult, error) {
	query := `SELECT id, storage_path FROM content.func_delete_orphan_media()`
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

	var results []orphanMediaResult
	for rows.Next() {
		var r orphanMediaResult
		if err := rows.Scan(&r.ID, &r.StoragePath); err == nil {
			results = append(results, r)
		}
	}
	return results, nil
}
