package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

type InboxFallbackResult struct {
	Conversation lite_models.ConvLiteRequest
	Member       lite_models.MemberLiteRequest
}

func FuncLoadConversationFallback(ctx context.Context, userID int64, convIDs []int64) ([]InboxFallbackResult, error) {
	query := `SELECT conversation_id, title, description, avatar_id, type, conversation_settings, last_message_id, role, settings, frozen_message_id, last_read_message_id, unread_count, joined_at FROM messaging.func_load_conversation_fallback($1, $2)`
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
		var convSettingsRaw sql.NullString
		var lastMsgID sql.NullInt64
		var role, unreadCount int
		var frozenID sql.NullInt64
		var lastReadMsgID sql.NullInt64
		var joinedAt time.Time
		var memSettingsRaw sql.NullString
		var externalLink sql.NullString

		if err := rows.Scan(&cid, &title, &description, &avatarID, &cType, &convSettingsRaw, &lastMsgID, &role, &memSettingsRaw, &externalLink, &frozenID, &lastReadMsgID, &unreadCount, &joinedAt); err == nil {
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
			if convSettingsRaw.Valid && convSettingsRaw.String != "" && convSettingsRaw.String != "{}" {
				_ = json.Unmarshal([]byte(convSettingsRaw.String), &conv.Settings)
			}
			if externalLink.Valid && externalLink.String != "" {
				_ = json.Unmarshal([]byte(externalLink.String), &conv.ExternalLink)
			}

			var parsedMemSettings member_models.MemberSettings
			if memSettingsRaw.Valid && memSettingsRaw.String != "" && memSettingsRaw.String != "{}" {
				_ = json.Unmarshal([]byte(memSettingsRaw.String), &parsedMemSettings)
			}

			mem := lite_models.MemberLiteRequest{
				ConversationID: cid,
				UserID:         userID,
				Role:           role,
				Settings: lite_models.MemberSettingsLite{
					IsMuted:           parsedMemSettings.IsMuted,
					MuteExpireAt:      parsedMemSettings.MuteExpireAt,
					Pinned:            parsedMemSettings.Pinned,
					MediaAutoDownload: parsedMemSettings.MediaAutoDownload,
				},
				UnreadCount: unreadCount,
				JoinedAt:    joinedAt.UnixMilli(),
			}
			if frozenID.Valid {
				mem.FrozenMessageID = frozenID.Int64
			}
			if lastReadMsgID.Valid {
				mem.LastReadMessageID = lastReadMsgID.Int64
			}
			results = append(results, InboxFallbackResult{Conversation: conv, Member: mem})
		}
	}
	return results, nil
}
