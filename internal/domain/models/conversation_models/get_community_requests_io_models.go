package conversation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"

type GetCommunityRequestsInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	Limit          int64 `json:"limit" binding:"omitempty,min=1,max=50"`
	Offset         int64 `json:"offset" binding:"omitempty,min=0"`
}

type GetCommunityRequestsOutput struct {
	Requests []auth_models.UserLiteView `json:"requests"`
}
