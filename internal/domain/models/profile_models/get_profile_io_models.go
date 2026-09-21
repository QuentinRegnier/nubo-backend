package profile_models

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/media_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
)

// GetProfileInput valide la demande du client.
// L'utilisation d'un JSON évite les paramètres d'URL dégueulasses.
type GetProfileInput struct {
	TargetID int64 `json:"target_id" binding:"omitempty"` // Si vide ou 0, on charge le profil de l'appelant
	Limit    int64 `json:"limit" binding:"omitempty,min=1,max=50"`
	Offset   int64 `json:"offset" binding:"omitempty,min=0"`
}

// GetProfileOutput est la super-structure agglomérant tous les domaines.
type GetProfileOutput struct {
	User                   auth_models.UserProfileView `json:"user"`
	Avatar                 media_models.MediaView      `json:"avatar"`
	RelationViewerToTarget int                         `json:"relation_viewer_to_target"`        // Ce que l'appelant pense de la cible (0, 1, 2, -1)
	RelationTargetToViewer int                         `json:"relation_target_to_viewer"`        // Ce que la cible pense de l'appelant (0, 1, 2, -1)
	DirectConversationID   int64                       `json:"direct_conversation_id,omitempty"` // 0 si aucune conversation existante
	Posts                  []post_models.GetPostOutput `json:"posts"`
	LikedPostIDs           []int64                     `json:"liked_post_ids"`
	SavedPostIDs           []int64                     `json:"saved_post_ids"`
}
