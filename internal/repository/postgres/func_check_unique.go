package postgres

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

func FuncCheckUnique(ctx context.Context, entity redis.EntityType, field string, value string) (bool, error) {
	target, err := Redis2Postgres(entity)
	if err != nil {
		return false, nubo_error.NewInternal(err) // Coupe-circuit immédiat
	}

	query := `SELECT views.func_check_unique($1, $2, $3, $4::text)`

	var exists bool

	err = postgres.PostgresDB.QueryRowContext(ctx, query, target.Schema, target.Table, field, value).Scan(&exists)

	return exists, err
}
