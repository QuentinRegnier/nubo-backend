package conversation_models

// PromoteMemberInput valide la demande de promotion d'un membre.
type PromoteMemberInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	TargetUserID   int64 `json:"target_user_id" binding:"required"`
}
