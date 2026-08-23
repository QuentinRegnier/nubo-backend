package conversation_models

type SyncInboxInput struct {
	ClientUpdatedAt int64 `json:"client_updated_at"` // Timestamp en millisecondes fourni par le client
}

type SyncInboxOutput struct {
	NeedUpdate    bool                    `json:"need_update"`
	ServerUpdated int64                   `json:"server_updated"`
	Conversations []InboxConversationView `json:"conversations,omitempty"` // Renvoyé uniquement si NeedUpdate = true
}
