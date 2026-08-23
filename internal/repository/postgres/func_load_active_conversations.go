package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncLoadActiveConversations récupère les métadonnées pour le Seeding
func FuncLoadActiveConversations(ctx context.Context) ([]models.ConvLiteRequest, error) {
	query := `SELECT id, type, title, last_message_id FROM messaging.func_load_active_conversations()`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(rows)

	var results []models.ConvLiteRequest
	for rows.Next() {
		var cid int64
		var cType int
		var title sql.NullString
		var lastMsgID sql.NullInt64

		if err := rows.Scan(&cid, &cType, &title, &lastMsgID); err == nil {
			meta := models.ConvLiteRequest{ID: cid, Type: cType}
			if title.Valid {
				meta.Title = title.String
			}
			if lastMsgID.Valid {
				meta.LastMessageID = lastMsgID.Int64
			}
			results = append(results, meta)
		}
	}
	return results, nil
}
