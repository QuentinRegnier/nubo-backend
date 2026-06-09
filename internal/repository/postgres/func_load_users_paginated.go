package postgres

import (
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncLoadUsersPaginated appelle la fonction SQL auth.func_load_users_paginated
func FuncLoadUsersPaginated(limit, offset int) ([]auth_models.UserPayload, error) {
	query := `SELECT * FROM auth.func_load_users_paginated($1, $2)`
	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur fermeture rows dans FuncLoadUsersPaginated:", err)
		}
	}(rows)

	var users []auth_models.UserPayload
	for rows.Next() {
		var u auth_models.UserPayload
		var pp sql.NullInt64
		var bio sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.FirstName, &u.LastName, &pp, &bio, &u.Grade); err == nil {
			if pp.Valid {
				u.ProfilePictureID = pp.Int64 // ✅ Typage correct en int64
			}
			if bio.Valid {
				u.Bio = bio.String
			}
			users = append(users, u)
		}
	}
	return users, nil
}
