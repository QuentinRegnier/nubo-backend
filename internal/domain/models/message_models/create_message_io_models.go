package message_models

// CreateMessageInput valide les données entrantes.
// On utilise `form` au lieu de `json` pour supporter le multipart/form-data (upload de fichiers).
type CreateMessageInput struct {
	ConversationID int64          `json:"conversation_id" binding:"required"`
	MessageType    int            `json:"message_type" binding:"required,min=0,max=8"`
	Content        string         `json:"content" binding:"omitempty,max=2200"`
	Attachments    map[string]any `json:"attachments" binding:"omitempty"` // NOUVEAU: Map JSON native
}

type CreateMessageOutput struct {
	MessageID int64 `json:"message_id"`
}
