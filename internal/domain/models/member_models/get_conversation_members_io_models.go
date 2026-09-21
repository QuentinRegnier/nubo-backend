package member_models

// GetConversationMembersInput valide la demande du client (POST JSON)
type GetConversationMembersInput struct {
	ConversationIDs []int64 `json:"conversation_ids" binding:"required,min=1,max=50"`
}

// ConversationMembersList groupe les membres par conversation pour que le front s'y retrouve facilement
type ConversationMembersList struct {
	ConversationID int64        `json:"conversation_id"`
	Members        []MemberView `json:"members"`
}
