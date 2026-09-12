package conversation_models

type GetUserConversationsInput struct {
	Limit  int64 `form:"limit,default=50"`
	Offset int64 `form:"offset,default=0"`
	Force  bool  `form:"force"`
}

type GetUserInboxOutput struct {
	Conversations []InboxConversationView `json:"conversations"`
}
