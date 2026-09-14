package conversation_models

import "time"

// AcceptCommunityRequestInput valide les données pour accepter une candidature.
type AcceptCommunityRequestInput struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
	TargetUserID   int64 `json:"target_user_id" binding:"required"`
}

type AcceptCommunityRequestOutput struct {
	InboxUpdateAt time.Time `json:"inbox_update_at"`
}
