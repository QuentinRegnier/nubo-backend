package member_models

// GetConversationMembersInput valide la demande du client avec pagination
type GetConversationMembersInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	Limit          int64 `json:"limit" binding:"min=1,max=100"`
	Offset         int64 `json:"offset" binding:"min=0"`
}

// GetConversationMembersOutput structure la réponse paginée
type GetConversationMembersOutput struct {
	ConversationID int64        `json:"conversation_id"`
	Members        []MemberView `json:"members"`
}
