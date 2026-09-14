package conversation_models

import "time"

type PinConversationInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}

type PinConversationOutput struct {
	InboxUpdateAt time.Time `json:"inbox_update_at"`
}
