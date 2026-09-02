package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/lib/pq"
)

// FullInboxResult est le type consolidé retournant les payloads complets depuis Postgres (L3)
type FullInboxResult struct {
	Conversation conversation_models.ConversationPayload
	Member       conversation_models.MemberPayload
}

// FuncLoadConversationPaginated appelle la fonction SQL de pagination de l'Inbox (L3) avec les Payloads complets
func FuncLoadConversationPaginated(ctx context.Context, userID int64, limit int64, offset int64) ([]FullInboxResult, error) {
	// Appel de la fonction SQL qui renvoie toutes les colonnes des deux tables
	query := `SELECT 
		conv_id, conv_type, conv_title, conv_last_msg_id, conv_state, conv_laws, conv_created, conv_updated, 
		mem_id, mem_conv_id, mem_user_id, mem_role, mem_joined, mem_unread, mem_frozen_id, mem_created, mem_updated 
		FROM messaging.func_load_conversation_paginated($1, $2, $3)`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	var results []FullInboxResult
	for rows.Next() {
		var c conversation_models.ConversationPayload
		var m conversation_models.MemberPayload
		var cTitle sql.NullString
		var cLastMsgID sql.NullInt64
		var memFrozenID sql.NullInt64

		err := rows.Scan(
			// Conversation
			&c.ID, &c.Type, &cTitle, &cLastMsgID, &c.State, pq.Array(&c.Laws), &c.CreatedAt, &c.UpdatedAt,
			// Member
			&m.ID, &m.ConversationID, &m.UserID, &m.Role, &m.JoinedAt, &m.UnreadCount, &memFrozenID, &m.CreatedAt, &m.UpdatedAt,
		)
		if err == nil {
			// Mapping sécurisé des NULL relationnels vers les zéro-values Go
			if cTitle.Valid {
				c.Title = cTitle.String
			}
			if cLastMsgID.Valid {
				c.LastMessageID = cLastMsgID.Int64
			}
			if memFrozenID.Valid {
				m.FrozenMessageID = memFrozenID.Int64
			}

			results = append(results, FullInboxResult{
				Conversation: c,
				Member:       m,
			})
		} else {
			logger.Log.Error().Err(err).Msg("Erreur de scan des lignes Postgres")
		}
	}
	return results, nil
}
