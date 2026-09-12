package conversation_models

type GetConversationsInput struct {
	ConversationIDs []int64 `json:"conversation_ids" binding:"required,min=1,max=50"`
}

type GetInboxOutput struct {
	Conversations []InboxConversationView `json:"conversations"`
}
