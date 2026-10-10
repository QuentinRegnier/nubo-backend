package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

type orphanLikeTarget struct {
	TargetType int
	TargetID   int64
}

// FuncDeleteOrphanLikes appelle la fonction SQL stockée pour purger les likes orphelins
// et récupère leurs identifiants pour répercuter le nettoyage sur le cache L2 (MongoDB).
func FuncDeleteOrphanLikes(ctx context.Context) ([]orphanLikeTarget, error) {
	query := `SELECT target_type, target_id FROM content.func_delete_orphan_likes()`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			numan_log.Error(ctx).Err(err).Msg("Erreur fermeture rows Garbage Collector Likes")
		}
	}(rows)

	var targets []orphanLikeTarget
	for rows.Next() {
		var t orphanLikeTarget
		if err := rows.Scan(&t.TargetType, &t.TargetID); err == nil {
			targets = append(targets, t)
		}
	}

	return targets, nil
}
