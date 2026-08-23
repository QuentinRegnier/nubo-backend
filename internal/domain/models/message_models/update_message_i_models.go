package message_models

// UpdateMessageInput valide le nouveau contenu du message.
type UpdateMessageInput struct {
	MessageID int64  `json:"message_id" binding:"required"`
	Content   string `json:"content" binding:"required,max=2200"`
}
