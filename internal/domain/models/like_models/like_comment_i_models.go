package like_models

// LikeCommentInput centralise toute la donnée de l'action pour un commentaire.
type LikeCommentInput struct {
	UserID    int64  `json:"-"` // Protégé par le JWT
	CommentID int64  `json:"comment_id" binding:"required"`
	Action    string `json:"action" binding:"required,oneof=like unlike"`
}
