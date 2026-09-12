package postgres

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncAddIndirectTagToPost appelle directement la fonction SQL compilée (Pur DDD)
// pour ajouter un tag indirect sans risque de Race Condition.
func FuncAddIndirectTagToPost(ctx context.Context, postID int64, tag string) error {
	query := `SELECT content.func_add_indirect_tag_to_post($1, $2)`

	_, err := postgres.PostgresDB.ExecContext(ctx, query, postID, tag)
	if err != nil {
		return nubo_error.NewInternal(err)
	}

	return nil
}
