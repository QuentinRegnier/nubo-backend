package message_models

// UpdateMessageInput valide le nouveau contenu du message.
type UpdateMessageInput struct {
	MessageID int64  `json:"message_id" binding:"required"`
	Content   string `json:"content" binding:"required,max=2200"`
}

type UpdateMessageOutput struct {
	InboxUpdateAt int64 `json:"inbox_update_at"`
}
