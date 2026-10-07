package postgres

import (
	"context"
	"database/sql"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/lib/pq"
)

// scanPosts mutualise la logique d'itération et de scan des lignes (DRY).
func scanPosts(ctx context.Context, rows *sql.Rows) ([]post_models.PostPayload, error) {
	var posts []post_models.PostPayload
	for rows.Next() {
		var p post_models.PostPayload
		var location sql.NullString

		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Content,
			pq.Array(&p.Hashtags),
			pq.Array(&p.IndirectHashtags), // ✅ NOUVEAU
			pq.Array(&p.Identifiers),
			pq.Array(&p.MediaIDs),
			&p.Visibility,
			&p.PriorityLevel,
			&location,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.LikeCount,
			&p.CommentCount,
			&p.ViewCount,
			&p.ReportCount,
			&p.HasMedia,
			pq.Array(&p.Vector),
			&p.VectorVersion,
			&p.TelemetryDwellSum,
			&p.TelemetryDwellSq,
			&p.TelemetryClicks,
		)

		if err != nil {
			nubo_log.Error(ctx).Err(err).Msg("Erreur lors du scan d'un post (Postgres)")
			continue // On ignore la ligne corrompue et on passe à la suivante
		}

		if location.Valid {
			p.Location = location.String
		}

		posts = append(posts, p)
	}

	if err := rows.Err(); err != nil {
		nubo_log.Error(ctx).Err(err).Msg("Erreur lors de l'itération sur les résultats SQL (rows.Err)")
		return nil, nubo_error.NewInternal()
	}

	return posts, nil
}
