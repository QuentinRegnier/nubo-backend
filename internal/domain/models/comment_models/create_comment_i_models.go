package comment_models

type CreateCommentInput struct {
	UserID  int64  `json:"-"` // Sécurisé par le Handler (JWT)
	PostID  int64  `json:"post_id" binding:"required"`
	Content string `json:"content" binding:"required,max=2200"`
}
