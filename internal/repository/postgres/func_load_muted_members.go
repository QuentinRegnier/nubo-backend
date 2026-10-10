package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

type mutedMemberRecord struct {
	UserID          int64
	RestrictedUntil int64
}

// FuncLoadMutedMembersPaginated appelle la fonction SQL pour lister les membres restreints
func FuncLoadMutedMembersPaginated(ctx context.Context, convID int64, limit int64, offset int64) ([]mutedMemberRecord, error) {
	rows, err := postgres.PostgresDB.QueryContext(ctx, "SELECT * FROM messaging.func_load_muted_members_paginated($1, $2, $3)", convID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			numan_log.Error(ctx).Err(err).Msg("Erreur fermeture rows FuncLoadMutedMembersPaginated")
		}
	}(rows)

	var records []mutedMemberRecord
	for rows.Next() {
		var r mutedMemberRecord
		if err := rows.Scan(&r.UserID, &r.RestrictedUntil); err == nil {
			records = append(records, r)
		}
	}
	return records, nil
}
