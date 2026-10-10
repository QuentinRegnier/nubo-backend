package post_models

type DeletePostInput struct {
	PostID int64 `json:"post_id" binding:"required"`
}
