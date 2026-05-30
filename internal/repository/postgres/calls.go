package postgres

import (
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/lib/pq"
)

// scanPosts mutualise la logique d'itération et de scan des lignes (DRY).
// Elle lit les 16 colonnes (incluant view_count et vector) pour construire les PostRequests.
func scanPosts(rows *sql.Rows) ([]models.PostRequest, error) {
	var posts []models.PostRequest

	for rows.Next() {
		var p models.PostRequest
		var location sql.NullString

		err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Content,
			pq.Array(&p.Hashtags),
			pq.Array(&p.Identifiers),
			pq.Array(&p.MediaIDs),
			&p.Visibility,
			&location,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.LikeCount,
			&p.CommentCount,
			&p.ViewCount,
			&p.HasMedia,
			pq.Array(&p.Vector),
			&p.VectorVersion,
		)

		if err != nil {
			fmt.Printf("⚠️ Erreur lors du scan d'un post_service : %v\n", err)
			continue // On ignore la ligne corrompue et on passe à la suivante
		}

		if location.Valid {
			p.Location = location.String
		}

		posts = append(posts, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erreur pendant l'itération des posts : %w", err)
	}

	return posts, nil
}
