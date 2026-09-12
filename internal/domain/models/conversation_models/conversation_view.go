package conversation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"

type InboxConversationView struct {
	ConversationID int64                    `json:"conversation_id"`
	Type           int                      `json:"type"`            // 0=MP, 1=Groupe, 2=Com. Privée, 3=Com. Publique
	Title          string                   `json:"title,omitempty"` // Plus de pointeur ! ("" si vide)
	Description    string                   `json:"description,omitempty"`
	AvatarID       int64                    `json:"avatar_id"` // NOUVEAU: L'avatar de la conversation (ou 0 si pas d'avatar)
	LastMessageID  int64                    `json:"last_message_id"`
	Role           int                      `json:"role"`         // 0=Membre, 1=Admin, 2=Propriétaire
	Settings       MemberSettings           `json:"settings"`     // Paramètres de notification (0=Tout, 1=Silencieux, 2=Muet)
	UnreadCount    int                      `json:"unread_count"` // Pastille de notification (0 = tout lu)
	Avatars        []media_models.MediaView `json:"avatars"`      // NOUVEAU: Les 1 ou 2 avatars calculés par notre tri !
	IsOnline       bool                     `json:"is_online"`    // NOUVEAU : Présence pour les MP
}
