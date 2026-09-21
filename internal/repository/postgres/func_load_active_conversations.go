package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

func FuncLoadActiveConversations(ctx context.Context) ([]lite_models.ConvLiteRequest, error) {
	query := `SELECT id, type, title, description, avatar_id, last_message_id, settings, external_link FROM messaging.func_load_active_conversations()`
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

	var results []lite_models.ConvLiteRequest
	for rows.Next() {
		var cid int64
		var cType int
		var title sql.NullString
		var lastMsgID sql.NullInt64
		var description sql.NullString
		var avatarID sql.NullInt64
		var settingsRaw sql.NullString
		var externalLinksRaw sql.NullString

		if err := rows.Scan(&cid, &cType, &title, &description, &avatarID, &lastMsgID, &settingsRaw, &externalLinksRaw); err == nil {
			meta := lite_models.ConvLiteRequest{ID: cid, Type: cType}
			if title.Valid {
				meta.Title = title.String
			}
			if description.Valid {
				meta.Description = description.String
			}
			if avatarID.Valid {
				meta.AvatarID = avatarID.Int64
			}
			if lastMsgID.Valid {
				meta.LastMessageID = lastMsgID.Int64
			}
			if settingsRaw.Valid && settingsRaw.String != "" && settingsRaw.String != "{}" {
				_ = json.Unmarshal([]byte(settingsRaw.String), &meta.Settings)
			}
			if externalLinksRaw.Valid && externalLinksRaw.String != "" {
				_ = json.Unmarshal([]byte(externalLinksRaw.String), &meta.ExternalLink)
			}
			results = append(results, meta)
		}
	}
	return results, nil
}
