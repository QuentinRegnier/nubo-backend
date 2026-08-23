package conversation_models

import "time"

type MemberPayload struct {
	ID              int64     `json:"id" bson:"id"`
	ConversationID  int64     `json:"conversation_id" bson:"conversation_id"`
	UserID          int64     `json:"user_id" bson:"user_id"`
	Role            int       `json:"role" bson:"role"`
	JoinedAt        time.Time `json:"joined_at" bson:"joined_at"`
	UnreadCount     int       `json:"unread_count" bson:"unread_count"`
	FrozenMessageID int64     `json:"frozen_message_id" bson:"frozen_message_id"` // ✅ NOUVEAU
	CreatedAt       time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" bson:"updated_at"`
}
