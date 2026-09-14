package message_models

import "time"

// DeleteMessageInput valide la demande de suppression de message
type DeleteMessageInput struct {
	MessageID int64 `json:"message_id" binding:"required"`
}

type DeleteMessageOutput struct {
	InboxUpdateAt time.Time `json:"inbox_update_at"`
}
