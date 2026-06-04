package comment_models

// Structure interne correspondant exactement au schéma Postgres content.comments
type CommentPayload struct {
	ID         int64  `json:"id"`
	PostID     int64  `json:"post_id"`
	UserID     int64  `json:"user_id"`
	Content    string `json:"content"`
	Visibility int    `json:"visibility"`
	LikeCount  int    `json:"like_count" bson:"like_count"` // <-- Ligne à ajouter
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}
