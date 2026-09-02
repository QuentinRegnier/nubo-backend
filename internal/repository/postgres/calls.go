package postgres

import (
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/lib/pq"
)

// scanPosts mutualise la logique d'itération et de scan des lignes (DRY).
// Elle lit les 16 colonnes (incluant view_count et vector) pour construire les PostRequests.
func scanPosts(rows *sql.Rows) ([]post_models.PostPayload, error) {
	var posts []post_models.PostPayload
	for rows.Next() {
		var p post_models.PostPayload
		var location sql.NullString
		err := rows.Scan(
			&p.ID, &p.UserID, &p.Content, pq.Array(&p.Hashtags), pq.Array(&p.Identifiers), pq.Array(&p.MediaIDs),
			&p.Visibility, &p.PriorityLevel, &location, &p.CreatedAt, &p.UpdatedAt, &p.LikeCount, &p.CommentCount,
			&p.ViewCount, &p.HasMedia, pq.Array(&p.Vector), &p.VectorVersion,
			&p.TelemetryDwellSum, &p.TelemetryDwellSq, &p.TelemetryClicks, // <-- NOUVEAU
		)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors du scan d'un post (Postgres)")
			continue // On ignore la ligne corrompue et on passe à la suivante
		}
		if location.Valid {
			p.Location = location.String
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	return posts, nil
}
