package conversation_models

// UnpinConversationInput valide l'action de désépingler une conversation.
type UnpinConversationInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}
