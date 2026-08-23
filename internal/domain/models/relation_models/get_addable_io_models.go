package relation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models"

// GetAddableInput valide les requêtes de pagination depuis la Query String de l'URL.
type GetAddableInput struct {
	Limit  int64 `form:"limit,default=50" binding:"min=1,max=100"`
	Offset int64 `form:"offset,default=0" binding:"min=0"`
	Force  bool  `form:"force"` // Peut-être forcé via /force
}

type GetAddableOutput struct {
	Users []models.UserLiteRequest `json:"users"`
}
