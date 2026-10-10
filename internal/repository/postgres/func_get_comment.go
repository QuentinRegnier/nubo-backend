package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

// FuncGetComment récupère l'intégralité d'un commentaire depuis L3 via sa fonction SQL dédiée.
func FuncGetComment(ctx context.Context, commentID int64) (comment_models.CommentPayload, error) {
	var c comment_models.CommentPayload

	query := `SELECT id, post_id, user_id, content, visibility, like_count, score, created_at, updated_at FROM content.func_get_comment($1)`
	err := postgres.PostgresDB.QueryRowContext(ctx, query, commentID).Scan(
		&c.ID, &c.PostID, &c.UserID, &c.Content, &c.Visibility, &c.LikeCount, &c.Score, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c, numan_error.NewNotFound("COMMENT_NOT_FOUND", "Commentaire introuvable.", err)
		}
		numan_log.Error(ctx).Err(err).Msg("Erreur SQL inattendue lors de la vérification de l'enregistrement")
		return c, numan_error.NewInternal()
	}

	return c, nil
}
