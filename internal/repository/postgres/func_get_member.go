package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncGetMember récupère l'intégralité d'un membre depuis L3
func FuncGetMember(ctx context.Context, convID int64, userID int64) (conversation_models.MemberPayload, error) {
	query := `SELECT id, conversation_id, user_id, role, joined_at, unread_count, frozen_message_id, created_at, updated_at FROM messaging.func_get_member($1, $2)`

	var m conversation_models.MemberPayload
	var frozenID sql.NullInt64

	err := postgres.PostgresDB.QueryRowContext(ctx, query, convID, userID).Scan(
		&m.ID, &m.ConversationID, &m.UserID, &m.Role, &m.JoinedAt, &m.UnreadCount, &frozenID, &m.CreatedAt, &m.UpdatedAt,
	)

	if err == nil {
		if frozenID.Valid {
			m.FrozenMessageID = frozenID.Int64
		}
	}

	return m, err
}
