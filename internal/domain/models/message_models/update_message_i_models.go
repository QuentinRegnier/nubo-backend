package message_models

import "time"

// UpdateMessageInput valide le nouveau contenu du message.
type UpdateMessageInput struct {
	MessageID int64  `json:"message_id" binding:"required"`
	Content   string `json:"content" binding:"required,max=2200"`
}

type UpdateMessageOutput struct {
	InboxUpdateAt time.Time `json:"inbox_update_at"`
}
