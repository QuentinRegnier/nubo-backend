package postgres

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// TimelineSeedPayload structure temporaire pour la reconstruction L1
type TimelineSeedPayload struct {
	PostID    int64
	UserID    int64
	CreatedAt time.Time
}

// FuncLoadTimelineSeedPaginated appelle la fonction SQL content.func_load_timeline_seed_paginated
func FuncLoadTimelineSeedPaginated(limit, offset int) ([]TimelineSeedPayload, error) {
	query := `SELECT * FROM content.func_load_timeline_seed_paginated($1, $2)`
	rows, err := postgres.PostgresDB.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("⚠️ Erreur fermeture rows dans FuncLoadTimelineSeedPaginated:", err)
		}
	}(rows)

	var seeds []TimelineSeedPayload
	for rows.Next() {
		var s TimelineSeedPayload
		if err := rows.Scan(&s.PostID, &s.UserID, &s.CreatedAt); err == nil {
			seeds = append(seeds, s)
		}
	}
	return seeds, nil
}
