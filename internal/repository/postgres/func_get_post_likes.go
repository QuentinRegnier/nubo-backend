package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncGetPostLikes utilise ta fonction polymorphe 'func_load_likes' pour récupérer les IDs.
func FuncGetPostLikes(ctx context.Context, postID int64, limit int, offset int) ([]int64, error) {

	// ⚠️ IMPORTANT : Remplace '0::smallint' par la valeur exacte de target_type qui correspond
	// à "Post" dans ton architecture (ex: si Post = 1, met 1::smallint).
	query := `
		SELECT user_id 
		FROM content.func_load_likes(
			0::smallint,   -- p_target_type (Filtre par type d'entité)
			$1::bigint,    -- p_target_id   (L'ID du post)
			NULL::bigint,  -- p_user_id     (On veut tous les utilisateurs)
			NULL::integer, -- p_limit       (Désactive la limite interne de la fonction)
			0::smallint    -- p_order_mode  (0 = DESC, les plus récents en premier)
		)
		LIMIT $2 OFFSET $3;
	`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, postID, limit, offset)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	var userIDs []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err == nil {
			userIDs = append(userIDs, uid)
		}
	}

	return userIDs, nil
}
