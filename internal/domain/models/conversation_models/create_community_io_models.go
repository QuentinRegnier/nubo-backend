package conversation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models"

// CreateCommunityInput valide les données pour la création d'une communauté publique (Type 3).
type CreateCommunityInput struct {
	Title        string               `json:"title" binding:"required,min=3,max=100"`
	Settings     ConversationSettings `json:"settings"`
	ExternalLink models.ExternalLinks `json:"external_link" binding:"omitempty"` // ✅ NOUVEAU
	OwnerID      int64                `json:"owner_id" binding:"omitempty"`
}

// CreateCommunityOutput renvoie l'ID généré.
type CreateCommunityOutput struct {
	ConversationID int64 `json:"conversation_id"`
	InboxUpdateAt  int64 `json:"conversation_update_at"`
}
