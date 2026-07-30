package saved_models

import "time"

// SavedPayload représente l'association entre un utilisateur et un post sauvegardé.
type SavedPayload struct {
	ID        int64     `json:"id" bson:"id"`
	UserID    int64     `json:"user_id" bson:"user_id"`
	PostID    int64     `json:"post_id" bson:"post_id"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}
