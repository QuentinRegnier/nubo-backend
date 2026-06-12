package post_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"

// GetPostInput représente la requête pour récupérer un ou plusieurs posts par leurs IDs, avec le contexte de l'utilisateur pour la validation d'accès.
type GetPostInput struct {
	UserID  int64   `json:"user_id"`
	PostIDs []int64 `json:"post_ids"`
}

// GetPostOutput représente la réponse pour un ID spécifique
type GetPostOutput struct {
	PostID   int64                             `json:"post_id"`
	Data     *PostPayload                      `json:"data,omitempty"`
	Media    []string                          `json:"media,omitempty"`    // ✅ Ajout du tableau d'URL signées prêtes à l'emploi
	Comments []comment_models.GetCommentOutput `json:"comments,omitempty"` // ✅ Ajout du tableau des commentaires (avec enveloppe d'erreur)
	Error    string                            `json:"nubo_error,omitempty"`
}
