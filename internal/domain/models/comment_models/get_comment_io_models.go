package comment_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"

type GetCommentsInput struct {
	UserID int64 `json:"user_id"` // Protégé, injecté dynamiquement par le middleware
	PostID int64 `form:"post_id" binding:"required"`
	Limit  int64 `form:"limit,default=50"`
	Offset int64 `form:"offset,default=0"`
}

// GetCommentOutput représente la réponse pour un ID spécifique
type GetCommentOutput struct {
	CommentID    int64                  `json:"comment_id"`
	Data         CommentPayload         `json:"data,omitempty"`
	AuthorAvatar media_models.MediaView `json:"author_avatar"` // <-- NOUVEAU (Par valeur)
	Error        string                 `json:"nubo_error,omitempty"`
}
