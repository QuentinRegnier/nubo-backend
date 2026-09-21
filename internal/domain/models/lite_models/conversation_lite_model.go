package lite_models

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
)

type ConversationSettings struct {
	JoinApprovalRequired bool  `bson:"join_approval_required" json:"join_approval_required" msgpack:"join_approval_required"`
	WritePermission      int   `bson:"write_permission" json:"write_permission" msgpack:"write_permission"`
	SendMediaPermission  bool  `bson:"send_media_permission" json:"send_media_permission" msgpack:"send_media_permission"`
	AddMemberPermission  bool  `bson:"add_member_permission" json:"add_member_permission" msgpack:"add_member_permission"`
	HideSystemMessages   bool  `bson:"hide_system_messages" json:"hide_system_messages" msgpack:"hide_system_messages"`
	JoinWithLinkDuration int64 `json:"join_with_link_duration" bson:"join_with_link_duration" msgpack:"join_with_link_duration"`
	SendSurveyPermission bool  `json:"send_survey_permission" bson:"send_survey_permission" msgpack:"send_survey_permission"`
}

type ConvLiteRequest struct {
	ID            int64                `bson:"id" json:"id"`
	Type          int                  `bson:"type" json:"type"`
	Title         string               `bson:"title" json:"title"`
	Description   string               `bson:"description" json:"description"`
	AvatarID      int64                `bson:"avatar_id" json:"avatar_id"`
	LastMessageID int64                `bson:"last_message_id" json:"last_message_id"`
	Settings      ConversationSettings `bson:"settings" json:"settings"` // Remplacement de Laws
	ExternalLink  models.ExternalLinks `bson:"external_links" json:"external_links"`
}
