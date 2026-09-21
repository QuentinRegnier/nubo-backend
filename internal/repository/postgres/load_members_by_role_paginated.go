package postgres

import (
	"context"
	"database/sql"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncLoadMembersByRolePaginated charge les membres d'une conversation par rôle (ex: -3 pour les requêtes)
func FuncLoadMembersByRolePaginated(ctx context.Context, convID int64, role int, limit int64, offset int64) ([]member_models.MemberPayload, error) {
	query := `SELECT * FROM messaging.func_load_members_by_role_paginated($1, $2, $3, $4)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, convID, role, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		if err := rows.Close(); err != nil {
			log.Printf("Erreur fermeture rows func_load_members_by_role_paginated: %v", err)
		}
	}(rows)

	var members []member_models.MemberPayload
	for rows.Next() {
		var mem member_models.MemberPayload
		var frozenDB sql.NullInt64 // Gestion du potentiel NULL

		err := rows.Scan(
			&mem.ID, &mem.ConversationID, &mem.UserID, &mem.Role,
			&mem.JoinedAt, &mem.UnreadCount, &frozenDB, &mem.CreatedAt, &mem.UpdatedAt,
		)
		if err == nil {
			if frozenDB.Valid {
				mem.FrozenMessageID = frozenDB.Int64
			}
			members = append(members, mem)
		}
	}
	return members, nil
}
