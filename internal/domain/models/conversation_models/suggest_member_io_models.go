package conversation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"

// SuggestMemberInput valide les critères de recherche pour l'ajout à une conversation.
type SuggestMemberInput struct {
	ConversationID int64  `json:"conversation_id" binding:"required"`
	Query          string `json:"query" binding:"omitempty,max=50"`
	Limit          int64  `json:"limit" binding:"omitempty,min=1,max=50"`
	Offset         int64  `json:"offset" binding:"omitempty,min=0"`
}

// SuggestMemberOutput renvoie les suggestions pré-filtrées et hydratées.
type SuggestMemberOutput struct {
	Users []auth_models.UserLiteView `json:"users"`
}
