package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/lib/pq"
)

// graphSeedPayload est une structure ultra-légère dédiée à l'initialisation de la mémoire
type graphSeedPayload struct {
	ID               int64
	Hashtags         []string
	IndirectHashtags []string // ✅ NOUVEAU
	CreatedAt        time.Time
}

// FuncLoadPostsForGraphSeeding ramène l'historique sémantique complet trié du plus vieux au plus récent
func FuncLoadPostsForGraphSeeding(ctx context.Context) ([]graphSeedPayload, error) {
	query := `SELECT id, hashtags, indirect_hashtags, created_at FROM content.func_load_posts_for_graph_seeding()`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			nubo_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	var seeds []graphSeedPayload
	for rows.Next() {
		var p graphSeedPayload
		if err := rows.Scan(&p.ID, pq.Array(&p.Hashtags), pq.Array(&p.IndirectHashtags), &p.CreatedAt); err == nil { // ✅ NOUVEAU (Scan correct)
			seeds = append(seeds, p)
		}
	}

	return seeds, nil
}
