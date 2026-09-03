package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/lib/pq"
)

type TimeDecayPost struct {
	ID            int64
	LikeCount     int
	CommentCount  int
	ViewCount     int
	HasMedia      bool
	CreatedAt     time.Time
	Hashtags      []string
	Visibility    int
	PriorityLevel int
	ReportCount   int
}

func FuncLoadPostsForTimeDecay(ctx context.Context, minAge, maxAge string) ([]TimeDecayPost, error) {
	query := `SELECT id, like_count, comment_count, view_count, has_media, created_at, hashtags, visibility, priority_level, report_count FROM content.func_load_posts_for_time_decay($1::interval, $2::interval)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, minAge, maxAge)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.Log.Error().Err(err).Msg("Erreur lors de la fermeture des lignes (Postgres)")
		}
	}(rows)

	var posts []TimeDecayPost
	for rows.Next() {
		var p TimeDecayPost
		err := rows.Scan(
			&p.ID, &p.LikeCount, &p.CommentCount, &p.ViewCount, &p.HasMedia,
			&p.CreatedAt, pq.Array(&p.Hashtags), &p.Visibility, &p.PriorityLevel, &p.ReportCount,
		)
		if err == nil {
			posts = append(posts, p)
		}
	}
	return posts, nil
}
