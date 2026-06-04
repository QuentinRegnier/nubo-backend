package like_models

// LikePostInput centralise toute la donnée de l'action.
type LikePostInput struct {
	UserID int64  `json:"user_id"`
	PostID int64  `json:"post_id"`
	Action string `json:"action" binding:"required,oneof=like unlike"`
}
