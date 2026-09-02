package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models" // ✅ Le bon import
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
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
			return m, nubo_error.NewNotFound("MEDIA_NOT_FOUND", "Média introuvable.", err)
		}
		return m, nubo_error.NewInternal(err)
	}

	// ✅ Rejet si le média a été supprimé (Soft-Delete)
	if !m.Visibility {
		return m, nubo_error.NewNotFound("MEDIA_DELETED", "Ce média a été supprimé.", nil)
	}

	return m, nil
}
