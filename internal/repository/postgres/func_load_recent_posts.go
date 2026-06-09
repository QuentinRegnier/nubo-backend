package postgres

import (
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

func FuncLoadRecentPosts(days int) ([]post_models.PostPayload, error) {
	query := `SELECT * FROM content.func_load_recent_posts($1)`

	rows, err := postgres.PostgresDB.Query(query, days)
	if err != nil {
		return nil, fmt.Errorf("erreur FuncLoadRecentPosts: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur lors de la fermeture des rows dans FuncLoadRecentPosts:", err)
		}
	}(rows)

	// NOUVEAU
	return scanPosts(rows)
}
