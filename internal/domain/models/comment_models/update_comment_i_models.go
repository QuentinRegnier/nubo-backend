package comment_models

type UpdateCommentInput struct {
	CommentID int64  `json:"comment_id" binding:"required"`
	Content   string `json:"content" binding:"required,max=2200"`
}
