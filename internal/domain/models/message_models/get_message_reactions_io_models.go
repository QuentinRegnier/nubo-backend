package message_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"

// GetMessageReactionsInput valide les paramètres de la requête pour le Slow Path.
type GetMessageReactionsInput struct {
	MessageID int64 `json:"message_id" binding:"required"`
	Limit     int   `json:"limit" binding:"min=1,max=100"`
	Offset    int   `json:"offset" binding:"min=0"`
}

// UserReactionView combine les infos lite d'un utilisateur et sa réaction.
type UserReactionView struct {
	User     auth_models.UserLiteView `json:"user"`
	Reaction string                   `json:"reaction"`
}

// GetMessageReactionsOutput structure la réponse de l'API.
type GetMessageReactionsOutput struct {
	MessageID int64              `json:"message_id"`
	Reactions []UserReactionView `json:"reactions"`
}
