package conversation_models

// ReadReceiptInput valide les données envoyées par l'application
type ReadReceiptInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}
