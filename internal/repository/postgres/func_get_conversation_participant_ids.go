package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncGetConversationParticipantIDs récupère la liste brute des IDs pour réhydrater le Speed Cache (L1)
func FuncGetConversationParticipantIDs(ctx context.Context, convID int64) ([]int64, error) {
	query := `SELECT user_id FROM messaging.func_get_conversation_participant_ids($1)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, convID)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
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
