package auth_models

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/lite_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
)

// UserLiteView compose les données du Speed Cache (UserLiteRequest)
// et l'URL signée du média générée à la volée.
type UserLiteView struct {
	User     lite_models.UserLiteRequest `json:"user"`
	Avatar   media_models.MediaView      `json:"avatar"`    // Composition stricte par valeur
	IsOnline bool                        `json:"is_online"` // NOUVEAU : Statut de présence
}
