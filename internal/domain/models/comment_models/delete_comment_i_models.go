package comment_models

type DeleteCommentInput struct {
	CommentID int64 `json:"comment_id" binding:"required"`
}
