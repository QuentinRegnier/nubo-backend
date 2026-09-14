package message_models

import "time"

// CreateMessageInput valide les données entrantes.
type CreateMessageInput struct {
	ConversationID int64          `json:"conversation_id" binding:"required"`
	MessageType    int            `json:"message_type" binding:"required,min=0,max=8"`
	Content        string         `json:"content" binding:"omitempty,max=2200"`
	Attachments    map[string]any `json:"attachments" binding:"omitempty"`
	ThreadParentID int64          `json:"thread_parent_id" binding:"omitempty"` // NOUVEAU
}

type CreateMessageOutput struct {
	MessageID     int64     `json:"message_id"`
	InboxUpdateAt time.Time `json:"inbox_update_at"`
}
