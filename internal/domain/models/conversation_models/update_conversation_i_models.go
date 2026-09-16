package conversation_models

// UpdateConversationInput valide les champs modifiables d'une conversation (PUT)
type UpdateConversationInput struct {
	ConversationID int64                `json:"conversation_id" binding:"required"`
	Title          string               `json:"title" binding:"max=100"`
	Settings       ConversationSettings `json:"settings"` // Remplacement de Laws
	Description    string               `json:"description"`
	AvatarID       int64                `json:"avatar_id"`
}

type UpdateConversationOutput struct {
	InboxUpdateAt int64 `json:"inbox_update_at"`
}
