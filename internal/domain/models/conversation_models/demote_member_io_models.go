package conversation_models

// DemoteMemberInput valide la demande de destitution d'un administrateur.
type DemoteMemberInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	TargetUserID   int64 `json:"target_user_id" binding:"required"`
}
