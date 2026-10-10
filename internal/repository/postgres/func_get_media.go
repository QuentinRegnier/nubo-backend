package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

func FuncGetMedia(ctx context.Context, mediaID int64) (media_models.MediaPayload, error) {
	query := `SELECT id, owner_id, storage_path, visibility, created_at, updated_at FROM content.get_media($1)`

	var m media_models.MediaPayload
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
			return m, numan_error.NewNotFound("MEDIA_NOT_FOUND", "Média introuvable.", err)
		}
		numan_log.Error(ctx).Err(err).Msg("Erreur SQL inattendue lors de la vérification de l'enregistrement")
		return m, numan_error.NewInternal()
	}

	// ✅ Rejet si le média a été supprimé (Soft-Delete)
	if !m.Visibility {
		return m, numan_error.NewNotFound("MEDIA_DELETED", "Ce média a été supprimé.", nil)
	}

	return m, nil
}
