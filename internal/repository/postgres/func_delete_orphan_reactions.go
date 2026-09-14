package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncDeleteOrphanReactions appelle la fonction SQL pour purger les réactions orphelines
// et retourne la liste des message_ids concernés.
func FuncDeleteOrphanReactions(ctx context.Context) ([]int64, error) {
	query := `SELECT message_id FROM messaging.func_delete_orphan_reactions()`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur fermeture rows Garbage Collector Reactions")
		}
	}(rows)

	var orphanMessageIDs []int64
	for rows.Next() {
		var messageID int64
		if err := rows.Scan(&messageID); err == nil {
			orphanMessageIDs = append(orphanMessageIDs, messageID)
		}
	}

	return orphanMessageIDs, nil
}
