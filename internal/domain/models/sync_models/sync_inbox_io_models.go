package sync_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"

type SyncInboxInput struct {
	ClientUpdatedAt int64 `json:"client_updated_at"` // Timestamp en millisecondes fourni par le client
}

type SyncInboxOutput struct {
	NeedUpdate    bool                                        `json:"need_update"`
	ServerUpdated int64                                       `json:"server_updated"`
	Conversations []conversation_models.InboxConversationView `json:"conversations,omitempty"` // Renvoyé uniquement si NeedUpdate = true
}
