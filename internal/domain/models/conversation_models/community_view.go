package conversation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"

// CommunityLiteView représente l'empreinte minimale d'une communauté pour la barre de recherche
type CommunityLiteView struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	Avatar      media_models.MediaView `json:"avatar"`
	Description string                 `json:"description"`
	MemberCount int                    `json:"member_count"`
}
