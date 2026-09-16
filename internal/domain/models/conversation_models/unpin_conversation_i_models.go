package conversation_models

// UnpinConversationInput valide l'action de désépingler une conversation.
type UnpinConversationInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}

type UnpinConversationOutput struct {
	InboxUpdateAt int64 `json:"inbox_update_at"`
}
