package postgres

import (
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncLoadAllTags récupère tous les slugs actifs depuis la base de données.
func FuncLoadAllTags() ([]string, error) {
	fmt.Println("FuncLoadAllTags called")

	sqlStatement := `SELECT slug FROM content.func_load_all_tags()`

	rows, err := postgres.PostgresDB.Query(sqlStatement)
	if err != nil {
		return nil, fmt.Errorf("erreur lors de l'exécution de FuncLoadAllTags: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur lors de la fermeture des rows dans FuncLoadAllTags:", err)
		}
	}(rows)

	var tags []string

	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err == nil {
			tags = append(tags, slug)
		} else {
			fmt.Printf("⚠️ Erreur lors du scan d'un tag : %v\n", err)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur pendant l'itération de FuncLoadAllTags: %w", err)
	}

	return tags, nil
}
