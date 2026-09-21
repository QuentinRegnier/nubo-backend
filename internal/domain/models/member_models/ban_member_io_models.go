package member_models

// BanMemberInput valide la demande de bannissement d'un membre.
type BanMemberInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	TargetUserID   int64 `json:"target_user_id" binding:"required"`
}

type BanMemberOutput struct {
	InboxUpdateAt int64 `json:"inbox_update_at"`
}
