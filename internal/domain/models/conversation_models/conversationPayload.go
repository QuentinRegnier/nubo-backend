package conversation_models

import (
	"time"
)

type ConversationSettings struct {
	JoinApprovalRequired bool `json:"join_approval_required" bson:"join_approval_required" msgpack:"join_approval_required"`
	WritePermission      int  `json:"write_permission" bson:"write_permission" msgpack:"write_permission"`
	SendMediaPermission  int  `json:"send_media_permission" bson:"send_media_permission" msgpack:"send_media_permission"`
	AddMemberPermission  int  `json:"add_member_permission" bson:"add_member_permission" msgpack:"add_member_permission"`
	HideSystemMessages   bool `json:"hide_system_messages" bson:"hide_system_messages" msgpack:"hide_system_messages"`
}

// ConversationPayload correspond exactement au schéma Postgres messaging.conversations
type ConversationPayload struct {
	ID            int64                `json:"id" bson:"id"`
	Type          int                  `json:"type" bson:"type"` // -2, -1, 0, 1, 2, 3
	Title         string               `json:"title" bson:"title"`
	Description   string               `json:"description" bson:"description"`
	AvatarID      int64                `json:"avatar_id" bson:"avatar_id"`
	LastMessageID int64                `json:"last_message_id" bson:"last_message_id"`
	State         int                  `json:"state" bson:"state"`       // 0=active, 1=archived
	Settings      ConversationSettings `json:"settings" bson:"settings"` // Remplacement de Laws []int
	CreatedAt     time.Time            `json:"created_at" bson:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at" bson:"updated_at"`
}
