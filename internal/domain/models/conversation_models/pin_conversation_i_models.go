package conversation_models

type PinConversationInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}

type PinConversationOutput struct {
	InboxUpdateAt int64 `json:"inbox_update_at"`
}
