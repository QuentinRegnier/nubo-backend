package lite_models

type MemberSettingsLite struct {
	IsMuted           int   `bson:"is_muted" json:"is_muted" msgpack:"is_muted"`
	MuteExpireAt      int64 `bson:"mute_expire_at" json:"mute_expire_at" msgpack:"mute_expire_at"`
	Pinned            int   `bson:"pinned" json:"pinned" msgpack:"pinned"`
	MediaAutoDownload bool  `bson:"media_auto_download" json:"media_auto_download" msgpack:"media_auto_download"`
	RestrictedUntil   int64 `json:"restricted_until" bson:"restricted_until" msgpack:"restricted_until"`
}

type MemberLiteRequest struct {
	ConversationID  int64              `bson:"conversation_id" json:"conversation_id" msgpack:"conversation_id"`
	UserID          int64              `bson:"user_id" json:"user_id" msgpack:"user_id"`
	Role            int                `bson:"role" json:"role" msgpack:"role"`
	Settings        MemberSettingsLite `bson:"settings" json:"settings" msgpack:"settings"`
	UnreadCount     int                `bson:"unread_count" json:"unread_count" msgpack:"unread_count"`
	FrozenMessageID int64              `bson:"frozen_message_id" json:"frozen_message_id" msgpack:"frozen_message_id"`
	JoinedAt        int64              `bson:"joined_at" json:"joined_at" msgpack:"joined_at"`
}
