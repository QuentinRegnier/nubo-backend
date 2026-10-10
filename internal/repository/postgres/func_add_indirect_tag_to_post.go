package postgres

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/infrastructure/postgres"
)

// FuncAddIndirectTagToPost appelle directement la fonction SQL compilée (Pur DDD)
// pour ajouter un tag indirect sans risque de Race Condition.
func FuncAddIndirectTagToPost(ctx context.Context, postID int64, tag string) error {
	query := `SELECT content.func_add_indirect_tag_to_post($1, $2)`

	_, err := postgres.PostgresDB.ExecContext(ctx, query, postID, tag)
	if err != nil {
		numan_log.Error(ctx).Err(err).Int64("post_id", postID).Str("tag", tag).Msg("Échec de l'exécution de la requête SQL (ExecContext)")
		return numan_error.NewInternal()
	}

	return nil
}
