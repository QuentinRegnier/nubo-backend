package postgres

import (
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/lib/pq"
)

func FuncLoadPosts(postIDs []int64, limit int, offset int) ([]post_models.PostPayload, error) {
	// 1. Préparation de l'argument des IDs
	var pPostIDs any
	if len(postIDs) > 0 {
		pPostIDs = pq.Array(postIDs)
	} else {
		// Très important : si le tableau est vide, on passe nil.
		// Postgres recevra NULL, ce qui validera la condition "p_post_ids IS NULL" de ta fonction SQL.
		pPostIDs = nil
	}

	// 2. Requête SQL alignée sur ta fonction :
	// func_load_posts(p_user_id, p_post_ids, p_visibility, p_order_mode)
	// On gère la pagination avec LIMIT et OFFSET à l'extérieur de la fonction SQL
	sqlStatement := `
		SELECT * FROM content.func_load_posts(
			NULL,  -- p_user_id (NULL = tous les utilisateurs)
			$1,    -- p_post_ids (NULL ou tableau d'IDs)
			NULL,  -- p_visibility (NULL = déclenche le DEFAULT ARRAY[0, 1] du SQL)
			0      -- p_order_mode (0 = plus récents)
		)
		LIMIT $2 OFFSET $3
	`

	// 3. Exécution de la requête
	rows, err := postgres.PostgresDB.Query(sqlStatement, pPostIDs, limit, offset)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes Postgres")
		}
	}(rows)

	// NOUVEAU : Un seul appel remplace toute la boucle
	return scanPosts(rows)
}
