package conversation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"

// SuggestInput unifie la recherche de contacts via un payload POST JSON propre.
type SuggestInput struct {
	ConversationID int64  `json:"conversation_id"` // 0 = Nouveau MP, > 0 = Groupe existant
	Query          string `json:"query" binding:"omitempty,max=50"`
	Limit          int64  `json:"limit" binding:"omitempty,min=1,max=50"`
	Offset         int64  `json:"offset" binding:"omitempty,min=0"`
}

type SuggestOutput struct {
	Users []auth_models.UserLiteView `json:"users"`
}
