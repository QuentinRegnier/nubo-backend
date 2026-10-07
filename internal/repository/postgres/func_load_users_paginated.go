package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncLoadUsersPaginated appelle la fonction SQL auth.func_load_users_paginated pour le Seeding L1
func FuncLoadUsersPaginated(ctx context.Context, limit, offset int) ([]lite_models.UserLiteRequest, error) {
	query := `SELECT id, username, first_name, last_name, profile_picture_id, bio, grade, conversation_permission, add_group_permission FROM auth.func_load_users_paginated($1, $2)`
	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Échec de l'exécution de la requête SQL (Query)")
		return nil, nubo_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			nubo_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	var users []lite_models.UserLiteRequest
	for rows.Next() {
		var u lite_models.UserLiteRequest
		var pp sql.NullInt64
		var bio sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.FirstName, &u.LastName, &pp, &bio, &u.Grade, &u.ConversationPermission, &u.AddGroupPermission); err == nil {
			if pp.Valid {
				u.ProfilePictureID = pp.Int64
			}
			if bio.Valid {
				u.Bio = bio.String
			}
			users = append(users, u)
		}
	}
	return users, nil
}
