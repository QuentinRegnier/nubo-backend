package postgres

import (
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncLoadUsersPaginated appelle la fonction SQL auth.func_load_users_paginated pour le Seeding L1
func FuncLoadUsersPaginated(limit, offset int) ([]models.UserLiteRequest, error) {
	query := `SELECT id, username, first_name, last_name, profile_picture_id, bio, grade, conversation_permission, add_group_permission FROM auth.func_load_users_paginated($1, $2)`
	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var users []models.UserLiteRequest
	for rows.Next() {
		var u models.UserLiteRequest
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
