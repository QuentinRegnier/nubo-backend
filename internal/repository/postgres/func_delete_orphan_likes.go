package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

type OrphanLikeTarget struct {
	TargetType int
	TargetID   int64
}

// FuncDeleteOrphanLikes appelle la fonction SQL stockée pour purger les likes orphelins
// et récupère leurs identifiants pour répercuter le nettoyage sur le cache L2 (MongoDB).
func FuncDeleteOrphanLikes(ctx context.Context) ([]OrphanLikeTarget, error) {
	query := `SELECT target_type, target_id FROM content.func_delete_orphan_likes()`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur fermeture rows Garbage Collector Likes")
		}
	}(rows)

	var targets []OrphanLikeTarget
	for rows.Next() {
		var t OrphanLikeTarget
		if err := rows.Scan(&t.TargetType, &t.TargetID); err == nil {
			targets = append(targets, t)
		}
	}

	return targets, nil
}
