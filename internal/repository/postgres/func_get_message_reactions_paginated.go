package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncGetMessageReactionsPaginated lit la liste des réactions d'un message depuis le L3
func FuncGetMessageReactionsPaginated(ctx context.Context, messageID int64, limit int, offset int) ([]message_models.MessageReactionPayload, error) {
	query := `SELECT id, message_id, user_id, reaction, created_at FROM messaging.func_get_message_reactions_paginated($1, $2, $3)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, messageID, limit, offset)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes Postgres (Reactions)")
		}
	}(rows)

	var reactions []message_models.MessageReactionPayload
	for rows.Next() {
		var r message_models.MessageReactionPayload
		if err := rows.Scan(&r.ID, &r.MessageID, &r.UserID, &r.Reaction, &r.CreatedAt); err == nil {
			reactions = append(reactions, r)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	return reactions, nil
}
