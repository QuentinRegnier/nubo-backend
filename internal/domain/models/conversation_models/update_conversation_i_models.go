package conversation_models

// UpdateConversationInput valide les champs modifiables d'une conversation (PUT)
type UpdateConversationInput struct {
	ConversationID int64  `json:"conversation_id" binding:"required"`
	Title          string `json:"title" binding:"max=100"`
	Laws           []int  `json:"laws"`
	Description    string `json:"description"`
	AvatarID       int64  `json:"avatar_id"`
}
