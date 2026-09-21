package member_models

// DemoteMemberInput valide la demande de destitution d'un administrateur.
type DemoteMemberInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	TargetUserID   int64 `json:"target_user_id" binding:"required"`
}

type DemoteMemberOutput struct {
	InboxUpdateAt int64 `json:"inbox_update_at"`
}
