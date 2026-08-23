package message_models

// DeleteMessageInput valide la demande de suppression de message
type DeleteMessageInput struct {
	MessageID int64 `json:"message_id" binding:"required"`
}
