package saved_models

type SaveActionInput struct {
	PostID int64 `json:"post_id" binding:"required"`
}
