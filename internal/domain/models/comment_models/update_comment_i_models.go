package comment_models

type UpdateCommentInput struct {
	UserID    int64  `json:"-"` // Sécurisé par le JWT
	CommentID int64  `json:"comment_id" binding:"required"`
	Content   string `json:"content" binding:"required,max=2200"`
}
