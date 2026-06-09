package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

// GraphSeedPayload est une structure ultra-légère dédiée à l'initialisation de la mémoire
type GraphSeedPayload struct {
	ID        int64
	Hashtags  []string
	CreatedAt time.Time
}

// FuncLoadPostsForGraphSeeding ramène l'historique sémantique complet trié du plus vieux au plus récent
func FuncLoadPostsForGraphSeeding(ctx context.Context) ([]GraphSeedPayload, error) {
	query := `SELECT id, hashtags, created_at FROM content.func_load_posts_for_graph_seeding()`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println("Error closing rows:", err)
		}
	}(rows)

	var seeds []GraphSeedPayload
	for rows.Next() {
		var p GraphSeedPayload
		if err := rows.Scan(&p.ID, pq.Array(&p.Hashtags), &p.CreatedAt); err == nil {
			seeds = append(seeds, p)
		}
	}
	return seeds, nil
}
