package like_models

type LikePayload struct {
	ID         int64  `json:"id"`
	TargetType int    `json:"target_type"`
	TargetID   int64  `json:"target_id"` // Remplace post_id pour la généricité
	UserID     int64  `json:"user_id"`
	CreatedAt  string `json:"created_at"`
}
