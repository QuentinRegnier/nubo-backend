package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncGetMember récupère l'intégralité d'un membre depuis L3
func FuncGetMember(ctx context.Context, convID int64, userID int64) (member_models.MemberPayload, error) {
	query := `SELECT id, conversation_id, user_id, role, settings, joined_at, unread_count, frozen_message_id, last_read_message_id, created_at, updated_at FROM messaging.func_get_member($1, $2)`

	var m member_models.MemberPayload
	var frozenID sql.NullInt64
	var lastID sql.NullInt64
	var settingsRaw sql.NullString

	err := postgres.PostgresDB.QueryRowContext(ctx, query, convID, userID).Scan(
		&m.ID, &m.ConversationID, &m.UserID, &m.Role, &settingsRaw, &m.JoinedAt, &m.UnreadCount, &frozenID, &lastID, &m.CreatedAt, &m.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return m, nubo_error.NewNotFound("MEMBER_NOT_FOUND", "Membre introuvable.", err)
		}
		return m, nubo_error.NewInternal(err)
	}

	if frozenID.Valid {
		m.FrozenMessageID = frozenID.Int64
	}
	if lastID.Valid {
		m.LastReadMessageID = lastID.Int64
	}

	// PARSING DU JSONB
	if settingsRaw.Valid && settingsRaw.String != "" && settingsRaw.String != "{}" {
		_ = json.Unmarshal([]byte(settingsRaw.String), &m.Settings)
	}

	return m, nil
}
