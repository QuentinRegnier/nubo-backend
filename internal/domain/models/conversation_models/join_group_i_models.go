package conversation_models

// JoinGroupInput valide la demande de l'utilisateur pour rejoindre un groupe.
type JoinGroupInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	InviteMsgID    int64 `json:"invite_msg_id" binding:"omitempty"` // Requis si la conversation est un groupe privé
}
