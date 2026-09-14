package conversation_models

import "time"

// UnpinConversationInput valide l'action de désépingler une conversation.
type UnpinConversationInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}

type UnpinConversationOutput struct {
	InboxUpdateAt time.Time `json:"inbox_update_at"`
}
