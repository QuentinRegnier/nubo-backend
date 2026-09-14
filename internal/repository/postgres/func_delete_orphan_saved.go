package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncDeleteOrphanSaved appelle la fonction SQL pour purger les favoris orphelins
// et retourne la liste des post_ids concernés.
func FuncDeleteOrphanSaved(ctx context.Context) ([]int64, error) {
	query := `SELECT post_id FROM content.func_delete_orphan_saved()`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur fermeture rows Garbage Collector Saved")
		}
	}(rows)

	var orphanPostIDs []int64
	for rows.Next() {
		var postID int64
		if err := rows.Scan(&postID); err == nil {
			orphanPostIDs = append(orphanPostIDs, postID)
		}
	}

	return orphanPostIDs, nil
}
