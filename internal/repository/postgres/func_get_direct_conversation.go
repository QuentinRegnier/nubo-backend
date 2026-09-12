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

// FuncGetDirectConversation interroge L3 pour trouver et ramener la conversation MP commune complète.
func FuncGetDirectConversation(ctx context.Context, user1, user2 int64) (conversation_models.ConversationPayload, error) {
	query := `SELECT id, type, title, description, avatar_id, last_message_id, state, laws, created_at, updated_at FROM messaging.func_get_direct_conversation($1, $2)`
	var c conversation_models.ConversationPayload
	var cTitle sql.NullString
	var cDescription sql.NullString
	var cAvatarID sql.NullInt64
	var cLastMsgID sql.NullInt64

	err := postgres.PostgresDB.QueryRowContext(ctx, query, user1, user2).Scan(
		&c.ID, &c.Type, &cTitle, &cDescription, &cAvatarID, &cLastMsgID, &c.State, pq.Array(&c.Laws), &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c, nubo_error.NewNotFound("CONV_NOT_FOUND", "Conversation privée introuvable.", err)
		}
		return c, nubo_error.NewInternal(err)
	}
	if cTitle.Valid {
		c.Title = cTitle.String
	}
	if cDescription.Valid {
		c.Description = cDescription.String
	}
	if cAvatarID.Valid {
		c.AvatarID = cAvatarID.Int64
	}
	if cLastMsgID.Valid {
		c.LastMessageID = cLastMsgID.Int64
	}
	return c, nil
}
