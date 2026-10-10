package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

func FuncLoadSavedPosts(ctx context.Context, userID int64, limit int64, offset int64) ([]saved_models.SavedPayload, error) {
	query := `SELECT id, user_id, post_id, created_at FROM content.func_load_saved_posts($1, $2, $3)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		numan_log.Error(ctx).Err(err).Msg("Échec de l'exécution de la requête SQL (QueryContext)")
		return nil, numan_error.NewInternal()
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			numan_log.Error(ctx).Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	var saveds []saved_models.SavedPayload
	for rows.Next() {
		var s saved_models.SavedPayload
		if err := rows.Scan(&s.ID, &s.UserID, &s.PostID, &s.CreatedAt); err == nil {
			saveds = append(saveds, s)
		}
	}
	return saveds, nil
}
