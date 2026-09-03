package lite_models

type MemberLiteRequest struct {
	ConversationID  int64 `bson:"conversation_id" json:"conversation_id"`
	UserID          int64 `bson:"user_id" json:"user_id"`
	UnreadCount     int   `bson:"unread_count" json:"unread_count"`
	Role            int   `bson:"role" json:"role"`
	FrozenMessageID int64 `bson:"frozen_message_id" json:"frozen_message_id"`
	JoinedAt        int64 `bson:"joined_at" json:"joined_at"` // AJOUT POUR LE TRI
}
