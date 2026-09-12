package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

type InboxFallbackResult struct {
	Conversation lite_models.ConvLiteRequest
	Member       lite_models.MemberLiteRequest
}

// FuncLoadConversationFallback appelle la fonction SQL pour réparer les trous du SPEED Cache (Inbox)
func FuncLoadConversationFallback(ctx context.Context, userID int64, convIDs []int64) ([]InboxFallbackResult, error) {
	query := `SELECT conversation_id, title, description, avatar_id, type, last_message_id, role, settings, unread_count, frozen_message_id, joined_at FROM messaging.func_load_conversation_fallback($1, $2)`
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
		var description sql.NullString
		var avatarID sql.NullInt64
		var cType int
		var lastMsgID sql.NullInt64
		var role, unreadCount int
		var frozenID sql.NullInt64
		var joinedAt time.Time // ✅ NOUVEAU
		var settings sql.NullString

		// ✅ NOUVEAU : On scanne la variable joinedAt à la fin
		if err := rows.Scan(&cid, &title, &description, &avatarID, &cType, &lastMsgID, &role, &settings, &unreadCount, &frozenID, &joinedAt); err == nil {
			conv := lite_models.ConvLiteRequest{ID: cid, Type: cType}
			if title.Valid {
				conv.Title = title.String
			}
			if description.Valid {
				conv.Description = description.String
			}
			if avatarID.Valid {
				conv.AvatarID = avatarID.Int64
			}
			if lastMsgID.Valid {
				conv.LastMessageID = lastMsgID.Int64
			}
			var parsedSettings conversation_models.MemberSettings
			if settings.Valid && settings.String != "" && settings.String != "{}" {
				_ = json.Unmarshal([]byte(settings.String), &parsedSettings)
			}

			mem := lite_models.MemberLiteRequest{
				ConversationID: cid,
				UserID:         userID,
				Role:           role,
				Settings: lite_models.MemberSettingsLite{
					IsMuted:           parsedSettings.IsMuted,
					MuteExpireAt:      parsedSettings.MuteExpireAt,
					Pinned:            parsedSettings.Pinned,
					MediaAutoDownload: parsedSettings.MediaAutoDownload,
				},
				UnreadCount: unreadCount,
				JoinedAt:    joinedAt.UnixMilli(), // ✅ MAGIE : la vraie date
			}
			if frozenID.Valid {
				mem.FrozenMessageID = frozenID.Int64
			}

			results = append(results, InboxFallbackResult{Conversation: conv, Member: mem})
		}
	}
	return results, nil
}
