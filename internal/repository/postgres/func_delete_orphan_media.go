package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

type OrphanMediaResult struct {
	ID          int64
	StoragePath string
}

// FuncDeleteOrphanMedia exécute la purge SQL et retourne les médias détruits.
func FuncDeleteOrphanMedia(ctx context.Context) ([]OrphanMediaResult, error) {
	query := `SELECT id, storage_path FROM content.func_delete_orphan_media()`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erreur purge orphan media: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("Erreur lors de la fermeture des lignes :", err)
		}
	}(rows)

	var results []OrphanMediaResult
	for rows.Next() {
		var r OrphanMediaResult
		if err := rows.Scan(&r.ID, &r.StoragePath); err == nil {
			results = append(results, r)
		}
	}
	return results, nil
}
