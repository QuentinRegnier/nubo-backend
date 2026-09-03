package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncLoadActiveConversations récupère les métadonnées pour le Seeding
func FuncLoadActiveConversations(ctx context.Context) ([]lite_models.ConvLiteRequest, error) {
	query := `SELECT id, type, title, last_message_id FROM messaging.func_load_active_conversations()`
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

		if err := rows.Scan(&cid, &cType, &title, &lastMsgID); err == nil {
			meta := lite_models.ConvLiteRequest{ID: cid, Type: cType}
			if title.Valid {
				meta.Title = title.String
			}
			if lastMsgID.Valid {
				meta.LastMessageID = lastMsgID.Int64
			}
			results = append(results, meta)
		}
	}
	return results, nil
}
