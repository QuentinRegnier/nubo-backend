package conversation_models

import "time"

type MemberSettings struct {
	IsMuted           bool  `json:"is_muted" bson:"is_muted" msgpack:"is_muted"`
	MuteExpireAt      int64 `json:"mute_expire_at" bson:"mute_expire_at" msgpack:"mute_expire_at"`
	Pinned            int   `json:"pinned" bson:"pinned" msgpack:"pinned"` // int au lieu de bool pour l'ordre d'épinglage
	MediaAutoDownload bool  `json:"media_auto_download" bson:"media_auto_download"`
}

type MemberPayload struct {
	ID              int64          `json:"id" bson:"id"`
	ConversationID  int64          `json:"conversation_id" bson:"conversation_id"`
	UserID          int64          `json:"user_id" bson:"user_id"`
	Role            int            `json:"role" bson:"role"`
	Settings        MemberSettings `json:"settings" bson:"settings"` // ✅ NOUVEAU
	JoinedAt        time.Time      `json:"joined_at" bson:"joined_at"`
	UnreadCount     int            `json:"unread_count" bson:"unread_count"`
	FrozenMessageID int64          `json:"frozen_message_id" bson:"frozen_message_id"`
	CreatedAt       time.Time      `json:"created_at" bson:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at" bson:"updated_at"`
}
