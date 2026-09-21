package conversation_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models"

type ConversationSettings struct {
	JoinApprovalRequired bool  `json:"join_approval_required" bson:"join_approval_required" msgpack:"join_approval_required"`
	WritePermission      int   `json:"write_permission" bson:"write_permission" msgpack:"write_permission"`
	SendMediaPermission  bool  `json:"send_media_permission" bson:"send_media_permission" msgpack:"send_media_permission"`
	AddMemberPermission  bool  `json:"add_member_permission" bson:"add_member_permission" msgpack:"add_member_permission"`
	HideSystemMessages   bool  `json:"hide_system_messages" bson:"hide_system_messages" msgpack:"hide_system_messages"`
	JoinWithLinkDuration int64 `json:"join_with_link_duration" bson:"join_with_link_duration" msgpack:"join_with_link_duration"`
	SendSurveyPermission bool  `json:"send_survey_permission" bson:"send_survey_permission" msgpack:"send_survey_permission"`
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
	ExternalLink  models.ExternalLinks `json:"external_link" bson:"external_link"`
	CreatedAt     int64                `json:"created_at" bson:"created_at"`
	UpdatedAt     int64                `json:"updated_at" bson:"updated_at"`
}

func DefaultConversationSettings(convType int) ConversationSettings {
	switch convType {
	case 3: // Communautés (Vitrine par défaut)
		return ConversationSettings{
			JoinApprovalRequired: false,
			WritePermission:      2,     // Annonces & Threads
			SendMediaPermission:  false, // Bloque les médias aux membres
			AddMemberPermission:  false, // Bloque l'ajout aux membres
			HideSystemMessages:   true,  // On cache le bruit de fond
		}
	default: // MP (Type 0) ou inconnu
		return ConversationSettings{
			JoinApprovalRequired: false,
			WritePermission:      0,
			SendMediaPermission:  true,
			AddMemberPermission:  true,
			HideSystemMessages:   false,
		}
	}
}
