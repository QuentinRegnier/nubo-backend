package conversation_models

type PinConversationInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}
