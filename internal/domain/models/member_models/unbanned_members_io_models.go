package member_models

// UnbanMembersInput gère le débannissement par lots
type UnbanMembersInput struct {
	ConversationID int64   `json:"conversation_id" binding:"required"`
	TargetUserIDs  []int64 `json:"target_user_ids" binding:"required,min=1,max=100"`
}

type UnbanMembersOutput struct {
	InboxUpdateAt int64 `json:"inbox_update_at"`
}
