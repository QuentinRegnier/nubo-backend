package like_models

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
)

type GetPostLikesInput struct {
	CallerID int64 `json:"caller_id"` // Injecté par le Handler (Sécurité)
	PostID   int64 `json:"post_id" binding:"required"`
	Limit    int   `json:"limit" binding:"min=1,max=100"`
	Offset   int   `json:"offset" binding:"min=0"`
}

type GetPostLikesOutput struct {
	PostID int64                      `json:"post_id"`
	Users  []auth_models.UserLiteView `json:"users"` // ✅ Remplacé : on renvoie les données prêtes à afficher
}
