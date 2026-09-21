package member_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"

// GetBannedMembersInput valide les paramètres du JSON pour récupérer la liste
type GetBannedMembersInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	Limit          int64 `json:"limit" binding:"omitempty,min=1,max=100"`
	Offset         int64 `json:"offset" binding:"omitempty,min=0"`
}

// BannedUserView enrichit le profil classique avec la date du bannissement
type BannedUserView struct {
	auth_models.UserLiteView
	BanDate int64 `json:"ban_date"` // Timestamp Unix (int64) correspondant à UpdatedAt du membre
}

type GetBannedMembersOutput struct {
	BannedUsers []BannedUserView `json:"banned_users"`
}
