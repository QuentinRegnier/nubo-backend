package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models" // ✅ Le bon import
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

func FuncGetMedia(ctx context.Context, mediaID int64) (models.MediaRequest, error) {
	query := `SELECT id, owner_id, storage_path, visibility, created_at, updated_at FROM content.get_media($1)`

	var m models.MediaRequest
	err := postgres.PostgresDB.QueryRowContext(ctx, query, mediaID).Scan(
		&m.ID,
		&m.OwnerID,
		&m.StoragePath,
		&m.Visibility,
		&m.CreatedAt,
		&m.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return m, errors.New("media not found")
		}
		return m, err
	}

	// ✅ Rejet si le média a été supprimé
	if !m.Visibility {
		return m, errors.New("media deleted")
	}

	return m, nil
}
