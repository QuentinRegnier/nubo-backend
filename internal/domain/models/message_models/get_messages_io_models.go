package message_models

// GetMessagesInput valide les paramètres de la requête pour charger l'historique
type GetMessagesInput struct {
	ConversationID int64  `form:"conversation_id" binding:"required"`
	Limit          int64  `form:"limit,default=50" binding:"max=100"`
	OffsetID       int64  `form:"offset_id,default=0"` // L'ID du message à partir duquel on scrolle
	Direction      string `form:"direction,default=top" binding:"oneof=top bottom"`
}

type GetMessagesOutput struct {
	Messages []MessagePayload `json:"messages"`
}
