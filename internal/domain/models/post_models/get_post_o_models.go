package post_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models"

// PostFetchResult représente la réponse pour un ID spécifique (Soit le post, soit une erreur d'accès).
type PostFetchResult struct {
	PostID int64               `json:"post_id"`
	Data   *models.PostRequest `json:"data,omitempty"`
	Error  string              `json:"error,omitempty"`
}
