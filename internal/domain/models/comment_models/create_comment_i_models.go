package comment_models

type CreateCommentInput struct {
	PostID  int64  `json:"post_id" binding:"required"`
	Content string `json:"content" binding:"required,max=2200"`
}
