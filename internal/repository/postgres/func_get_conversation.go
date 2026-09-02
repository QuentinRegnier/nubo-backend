package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

// FuncGetConversation récupère l'intégralité d'une conversation depuis L3
func FuncGetConversation(ctx context.Context, convID int64) (conversation_models.ConversationPayload, error) {
	query := `SELECT id, type, title, last_message_id, state, laws, created_at, updated_at FROM messaging.func_get_conversation($1)`

	var c conversation_models.ConversationPayload
	var cTitle sql.NullString
	var cLastMsgID sql.NullInt64

	err := postgres.PostgresDB.QueryRowContext(ctx, query, convID).Scan(
		&c.ID, &c.Type, &cTitle, &cLastMsgID, &c.State, pq.Array(&c.Laws), &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c, nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation introuvable.", err)
		}
		return c, nubo_error.NewInternal(err)
	}

	// Conversion des NULL SQL en Zéro-values Go
	if cTitle.Valid {
		c.Title = cTitle.String
	}
	if cLastMsgID.Valid {
		c.LastMessageID = cLastMsgID.Int64
	}

	return c, nil
}
