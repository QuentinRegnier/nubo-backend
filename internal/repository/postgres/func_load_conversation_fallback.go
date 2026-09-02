package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

type InboxFallbackResult struct {
	Conversation models.ConvLiteRequest
	Member       models.MemberLiteRequest
}

// FuncLoadConversationFallback appelle la fonction SQL pour réparer les trous du SPEED Cache (Inbox)
func FuncLoadConversationFallback(ctx context.Context, userID int64, convIDs []int64) ([]InboxFallbackResult, error) {
	query := `SELECT conversation_id, title, type, last_message_id, role, unread_count, frozen_message_id FROM messaging.func_load_conversation_fallback($1, $2)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID, convIDs)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var results []InboxFallbackResult
	for rows.Next() {
		var cid int64
		var title sql.NullString
		var cType int
		var lastMsgID sql.NullInt64
		var role, unreadCount int
		var frozenID sql.NullInt64

		if err := rows.Scan(&cid, &title, &cType, &lastMsgID, &role, &unreadCount, &frozenID); err == nil {
			conv := models.ConvLiteRequest{ID: cid, Type: cType}
			if title.Valid {
				conv.Title = title.String
			}
			if lastMsgID.Valid {
				conv.LastMessageID = lastMsgID.Int64
			}

			mem := models.MemberLiteRequest{
				ConversationID: cid,
				UserID:         userID,
				Role:           role,
				UnreadCount:    unreadCount,
			}
			if frozenID.Valid {
				mem.FrozenMessageID = frozenID.Int64
			}

			results = append(results, InboxFallbackResult{Conversation: conv, Member: mem})
		}
	}
	return results, nil
}
