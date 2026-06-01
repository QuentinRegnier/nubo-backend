package post_models

type GetPostLikesInput struct {
	CallerID int64 `json:"caller_id"` // Injecté par le Handler (Sécurité)
	PostID   int64 `json:"post_id" binding:"required"`
	Limit    int   `json:"limit" binding:"min=1,max=100"`
	Offset   int   `json:"offset" binding:"min=0"`
}

type GetPostLikesOutput struct {
	PostID  int64   `json:"post_id"`
	UserIDs []int64 `json:"user_ids"`
}
