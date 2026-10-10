package member_models

import (
	"github.com/QuentinRegnier/numan-backend/internal/domain/models/auth_models"
)

// GetMutedMembersInput valide la requête POST (URL propre).
type GetMutedMembersInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	Limit          int64 `json:"limit" binding:"omitempty,min=1,max=100"`
	Offset         int64 `json:"offset" binding:"omitempty,min=0"`
}

// MutedUserView combine les infos Lite d'un utilisateur et le timestamp de sa punition.
type MutedUserView struct {
	auth_models.UserLiteView
	RestrictedUntil int64 `json:"restricted_until"`
}

type GetMutedMembersOutput struct {
	MutedUsers []MutedUserView `json:"muted_users"`
}
