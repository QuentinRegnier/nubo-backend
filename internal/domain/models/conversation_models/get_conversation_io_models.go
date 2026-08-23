package conversation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"

type GetConversationInput struct {
	Limit  int64 `form:"limit,default=50"`
	Offset int64 `form:"offset,default=0"`
	Force  bool  `form:"force"`
}

type InboxConversationView struct {
	ConversationID int64                    `json:"conversation_id"`
	Type           int                      `json:"type"`            // 0=MP, 1=Groupe, 2=Com. Privée, 3=Com. Publique
	Title          string                   `json:"title,omitempty"` // Plus de pointeur ! ("" si vide)
	LastMessageID  int64                    `json:"last_message_id"`
	Role           int                      `json:"role"`         // 0=Membre, 1=Admin, 2=Propriétaire
	UnreadCount    int                      `json:"unread_count"` // Pastille de notification (0 = tout lu)
	Avatars        []media_models.MediaView `json:"avatars"`      // NOUVEAU: Les 1 ou 2 avatars calculés par notre tri !
}

type GetInboxOutput struct {
	Conversations []InboxConversationView `json:"conversations"`
}
