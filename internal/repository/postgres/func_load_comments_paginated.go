package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
)

// FuncLoadCommentsPaginated lit L3 et trie par pertinence
func FuncLoadCommentsPaginated(ctx context.Context, postID int64, offset int64, limit int64) ([]comment_models.CommentPayload, error) {
	var comments []comment_models.CommentPayload

	query := `SELECT id, post_id, user_id, content, visibility, like_count, created_at, updated_at FROM content.func_load_comments_paginated($1, $2, $3)`
	rows, err := postgres.PostgresDB.QueryContext(ctx, query, postID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(rows)

	for rows.Next() {
		var c comment_models.CommentPayload
		if err := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Content, &c.Visibility, &c.LikeCount, &c.CreatedAt, &c.UpdatedAt); err == nil {
			comments = append(comments, c)
		}
	}
	return comments, nil
}
