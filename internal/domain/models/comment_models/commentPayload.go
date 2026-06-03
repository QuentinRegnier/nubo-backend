package comment_models

// Structure interne correspondant exactement au schéma Postgres content.comments
type CommentPayload struct {
	ID         int64  `json:"id"`
	PostID     int64  `json:"post_id"`
	UserID     int64  `json:"user_id"`
	Content    string `json:"content"`
	Visibility int    `json:"visibility"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
