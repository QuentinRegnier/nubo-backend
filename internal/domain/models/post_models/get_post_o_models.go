package post_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models"

// GetPostInput représente la requête pour récupérer un ou plusieurs posts par leurs IDs, avec le contexte de l'utilisateur pour la validation d'accès.
type GetPostInput struct {
	UserID  int64   `json:"user_id"`
	PostIDs []int64 `json:"post_ids"`
}

// GetPostOutput représente la réponse pour un ID spécifique (Soit le post, soit une erreur d'accès).
type GetPostOutput struct {
	PostID int64               `json:"post_id"`
	Data   *models.PostRequest `json:"data,omitempty"`
	Error  string              `json:"error,omitempty"`
}
