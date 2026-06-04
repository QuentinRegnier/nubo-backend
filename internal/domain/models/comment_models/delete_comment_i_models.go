package comment_models

type DeleteCommentInput struct {
	UserID    int64 `json:"-"` // Sécurisé, injecté par le Handler via le JWT
	CommentID int64 `json:"comment_id" binding:"required"`
}
