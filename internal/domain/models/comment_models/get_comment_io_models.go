package comment_models

type GetCommentsInput struct {
	UserID int64 `json:"user_id"` // ✅ Protégé, injecté dynamiquement par le middleware
	PostID int64 `form:"post_id" binding:"required"`
	Limit  int64 `form:"limit,default=50"`
	Offset int64 `form:"offset,default=0"`
}

// GetCommentOutput représente la réponse pour un ID spécifique (Soit le comment, soit une erreur d'accès).
type GetCommentOutput struct {
	CommentID int64           `json:"comment_id"` // ✅ Correction du tag JSON
	Data      *CommentPayload `json:"data,omitempty"`
	Error     string          `json:"nubo_error,omitempty"`
}
