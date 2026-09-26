package sync_models

type FocusConversationInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	Active         bool  `json:"active"` // true = est entré sur l'écran, false = en est sorti
}
