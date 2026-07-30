package postgres

import (
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

func FuncLoadPostsPaginated(limit int, offset int) ([]post_models.PostPayload, error) {
	// Le SQL est maintenant encapsulé et pur
	query := `SELECT * FROM content.func_load_posts_paginated($1, $2)`

	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de FuncLoadPostsPaginated: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur lors de la fermeture des rows dans FuncLoadPostsPaginated:", err)
		}
	}(rows)

	// NOUVEAU
	return scanPosts(rows)
}
