package post_models

type DeletePostInput struct {
	UserID int64 `json:"user_id"`
	PostID int64 `json:"post_id" binding:"required"`
}
