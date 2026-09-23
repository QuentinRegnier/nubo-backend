package sync_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/message_models"

// SyncMessagesInput valide la requête POST entrante.
// Zéro paramètre d'URL : l'ID de conversation et le timestamp sont dans le JSON.
type SyncMessagesInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	SinceMs        int64 `json:"since_ms" binding:"required,min=0"`
}

// SyncMessagesOutput structure la réponse avec le DTO MessageView classique
type SyncMessagesOutput struct {
	Messages []message_models.MessageView `json:"messages"`
}
