package conversation_models

import "time"

// CreateCommunityInput valide les données pour la création d'une communauté publique (Type 3).
type CreateCommunityInput struct {
	Title    string               `json:"title" binding:"required,min=3,max=100"`
	Settings ConversationSettings `json:"settings"` // Remplacement de Laws
	OwnerID  int64                `json:"owner_id" binding:"omitempty"`
}

// CreateCommunityOutput renvoie l'ID généré.
type CreateCommunityOutput struct {
	ConversationID int64     `json:"conversation_id"`
	InboxUpdateAt  time.Time `json:"conversation_update_at"`
}
