package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/like_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
)

// FuncLoadLikes charge les objets complets de likes pour permettre l'auto-guérison L2.
// On passe <= 0 pour targetID ou userID si on ne souhaite pas filtrer sur ces champs.
func FuncLoadLikes(ctx context.Context, targetType int, targetID int64, userID int64, limit int, orderMode int) ([]like_models.LikePayload, error) {
	// Préparation des paramètres pour gérer les valeurs NULL (le fallback par défaut de la fonction SQL)
	var pTargetType, pTargetID, pUserID any

	if targetType >= 0 {
		pTargetType = targetType
	}
	if targetID > 0 {
		pTargetID = targetID
	}
	if userID > 0 {
		pUserID = userID
	}

	query := `SELECT id, target_type, target_id, user_id, created_at 
	          FROM content.func_load_likes($1, $2, $3, $4, $5)`

	rows, err := postgres.PostgresDB.QueryContext(ctx, query, pTargetType, pTargetID, pUserID, limit, orderMode)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres Likes)")
		}
	}(rows)

	var likes []like_models.LikePayload
	for rows.Next() {
		var l like_models.LikePayload
		var createdAt time.Time

		if err := rows.Scan(&l.ID, &l.TargetType, &l.TargetID, &l.UserID, &createdAt); err == nil {
			// Le Payload Go attend une string pour la date
			l.CreatedAt = createdAt.UTC().Format(time.RFC3339)
			likes = append(likes, l)
		}
	}

	return likes, nil
}
