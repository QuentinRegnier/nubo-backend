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

type ActiveMemberResult struct {
	Member        lite_models.MemberLiteRequest
	LastMessageID sql.NullInt64
}

// FuncLoadActiveMembers récupère les membres pour le Seeding de l'Inbox
func FuncLoadActiveMembers(ctx context.Context) ([]ActiveMemberResult, error) {
	query := `SELECT conversation_id, user_id, role, settings, unread_count, last_message_id, joined_at FROM messaging.func_load_active_members()`
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
		var settingsRaw sql.NullString
		var lastMsgID sql.NullInt64
		var joinedAt time.Time

		if err := rows.Scan(&cid, &uid, &role, &settingsRaw, &unreadCount, &lastMsgID, &joinedAt); err == nil {

			// PARSING DU JSONB VERS LA STRUCT GO
			var parsedSettings conversation_models.MemberSettings
			if settingsRaw.Valid && settingsRaw.String != "" && settingsRaw.String != "{}" {
				_ = json.Unmarshal([]byte(settingsRaw.String), &parsedSettings)
			}

			mem := lite_models.MemberLiteRequest{
				ConversationID: cid,
				UserID:         uid,
				Role:           role,
				Settings: lite_models.MemberSettingsLite{
					IsMuted:           parsedSettings.IsMuted,
					MuteExpireAt:      parsedSettings.MuteExpireAt,
					Pinned:            parsedSettings.Pinned,
					MediaAutoDownload: parsedSettings.MediaAutoDownload,
				},
				UnreadCount: unreadCount,
				JoinedAt:    joinedAt.UnixMilli(),
			}
			results = append(results, ActiveMemberResult{Member: mem, LastMessageID: lastMsgID})
		}
	}
	return results, nil
}
