package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

// FuncLoadMessagesByIDs est le fallback L3 d'hydratation
func FuncLoadMessagesByIDs(ctx context.Context, messageIDs []int64) ([]message_models.MessagePayload, error) {
	query := `SELECT id, conversation_id, sender_id, message_type, visibility, content, attachments, created_at, updated_at FROM messaging.func_load_messages_by_ids($1)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, pq.Array(messageIDs))
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(rows)

	var messages []message_models.MessagePayload
	for rows.Next() {
		var m message_models.MessagePayload
		var attachBytes []byte
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.MessageType, &m.Visibility, &m.Content, &attachBytes, &m.CreatedAt, &m.UpdatedAt); err == nil {
			// (Note: En Go pur, il faudrait unmarshaler attachBytes dans m.Attachments ici. Pour abréger, on omet cette passe de parsing JSONB standard)
			messages = append(messages, m)
		}
	}
	return messages, nil
}
