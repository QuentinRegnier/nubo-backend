package lite_models

type ConversationSettingsLite struct {
	JoinApprovalRequired bool `bson:"join_approval_required" json:"join_approval_required" msgpack:"join_approval_required"`
	WritePermission      int  `bson:"write_permission" json:"write_permission" msgpack:"write_permission"`
	SendMediaPermission  int  `bson:"send_media_permission" json:"send_media_permission" msgpack:"send_media_permission"`
	AddMemberPermission  int  `bson:"add_member_permission" json:"add_member_permission" msgpack:"add_member_permission"`
	HideSystemMessages   bool `bson:"hide_system_messages" json:"hide_system_messages" msgpack:"hide_system_messages"`
}

type ConvLiteRequest struct {
	ID            int64                    `bson:"id" json:"id"`
	Type          int                      `bson:"type" json:"type"`
	Title         string                   `bson:"title" json:"title"`
	Description   string                   `bson:"description" json:"description"`
	AvatarID      int64                    `bson:"avatar_id" json:"avatar_id"`
	LastMessageID int64                    `bson:"last_message_id" json:"last_message_id"`
	Settings      ConversationSettingsLite `bson:"settings" json:"settings"` // Remplacement de Laws
}
