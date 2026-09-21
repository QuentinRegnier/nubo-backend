package member_models

// RefuseCommunityRequestInput valide les données pour refuser une candidature.
type RefuseCommunityRequestInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	TargetUserID   int64 `json:"target_user_id" binding:"required"`
}

type RefuseCommunityRequestOutput struct {
	InboxUpdateAt int64 `json:"inbox_update_at"`
}
