package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

type InboxFallbackResult struct {
	Conversation models.ConvLiteRequest
	Member       models.MemberLiteRequest
}

// FuncLoadConversationFallback appelle la fonction SQL pour réparer les trous du SPEED Cache (Inbox)
func FuncLoadConversationFallback(ctx context.Context, userID int64, convIDs []int64) ([]InboxFallbackResult, error) {
	query := `SELECT conversation_id, title, type, last_message_id, role, unread_count, frozen_message_id FROM messaging.func_load_conversation_fallback($1, $2)`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID, pq.Array(convIDs))
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
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
