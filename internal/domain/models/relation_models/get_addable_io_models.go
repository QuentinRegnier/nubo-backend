package relation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"

type GetAddableInput struct {
	Limit  int64 `form:"limit,default=50" binding:"min=1,max=100"`
	Offset int64 `form:"offset,default=0" binding:"min=0"`
	Force  bool  `form:"force"` // Peut-être forcé via /force
}

type GetAddableOutput struct {
	Users []auth_models.UserLiteView `json:"users"` // <-- UTILISATION DU NOUVEAU DTO PAR VALEUR
}
