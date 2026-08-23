package message_models

// UnreactMessageInput valide la demande de retrait de réaction.
type UnreactMessageInput struct {
	MessageID int64 `json:"message_id" binding:"required"`
}
