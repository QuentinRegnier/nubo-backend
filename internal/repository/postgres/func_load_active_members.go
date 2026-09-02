package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

type ActiveMemberResult struct {
	Member        models.MemberLiteRequest
	LastMessageID sql.NullInt64
}

// FuncLoadActiveMembers récupère les membres pour le Seeding de l'Inbox
func FuncLoadActiveMembers(ctx context.Context) ([]ActiveMemberResult, error) {
	query := `SELECT conversation_id, user_id, role, unread_count, last_message_id FROM messaging.func_load_active_members()`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var results []ActiveMemberResult
	for rows.Next() {
		var cid, uid int64
		var role, unreadCount int
		var lastMsgID sql.NullInt64

		if err := rows.Scan(&cid, &uid, &role, &unreadCount, &lastMsgID); err == nil {
			mem := models.MemberLiteRequest{
				ConversationID: cid,
				UserID:         uid,
				Role:           role,
				UnreadCount:    unreadCount,
			}
			results = append(results, ActiveMemberResult{Member: mem, LastMessageID: lastMsgID})
		}
	}
	return results, nil
}
