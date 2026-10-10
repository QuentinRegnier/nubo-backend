package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

// FuncGetConversationParticipantIDs récupère la liste brute des IDs pour réhydrater le Speed Cache (L1)
func FuncGetConversationParticipantIDs(ctx context.Context, convID int64) ([]int64, error) {
	query := `SELECT user_id FROM messaging.func_get_conversation_participant_ids($1)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, convID)
	if err != nil {
		return nil, numan_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			numan_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
