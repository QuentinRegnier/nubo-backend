package conversation_models

// ReadReceiptInput valide les données envoyées par l'application
type ReadReceiptInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}

// ReadReceiptOutput renvoie le timestamp de synchronisation de l'inbox
type ReadReceiptOutput struct {
	InboxUpdateAt int64 `json:"inbox_updated_at"`
}
