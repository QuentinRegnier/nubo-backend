package user_settings_models

import "time"

type PrivacySettings struct {
	ProfileVisibility      int  `json:"profile_visibility" bson:"profile_visibility"`           // 0: Public, 1: Abonnés, 2: Amis
	PostVisibilityDefault  int  `json:"post_visibility_default" bson:"post_visibility_default"` // Visibilité par défaut à la création
	ConversationPermission int  `json:"conversation_permission" bson:"conversation_permission"` // 0: Tout le monde, 1: Abonnés, 2: Amis
	AllowTagging           int  `json:"allow_tagging" bson:"allow_tagging"`                     // Qui peut m'identifier (photo)
	AllowMentions          int  `json:"allow_mentions" bson:"allow_mentions"`                   // Qui peut me mentionner (@pseudo)
	ShowOnlineStatus       bool `json:"show_online_status" bson:"show_online_status"`           // Afficher le point vert
	ShowLocation           bool `json:"show_location" bson:"show_location"`                     // Afficher la ville sur le profil
	SearchByEmailPhone     bool `json:"search_by_email_phone" bson:"search_by_email_phone"`     // Trouvable via contact
	AllowContentSharing    bool `json:"allow_content_sharing" bson:"allow_content_sharing"`     // Les autres peuvent partager mes posts
}

type NotificationSettings struct {
	MasterPushEnabled   bool `json:"master_push_enabled" bson:"master_push_enabled"`
	MasterEmailEnabled  bool `json:"master_email_enabled" bson:"master_email_enabled"`
	NotifyNewFollower   bool `json:"notify_new_follower" bson:"notify_new_follower"`
	NotifyFriendRequest bool `json:"notify_friend_request" bson:"notify_friend_request"`
	NotifyMessages      bool `json:"notify_messages" bson:"notify_messages"`
	NotifyLikes         bool `json:"notify_likes" bson:"notify_likes"`
	NotifyComments      bool `json:"notify_comments" bson:"notify_comments"`
	NotifyMentions      bool `json:"notify_mentions" bson:"notify_mentions"`
	QuietHoursEnabled   bool `json:"quiet_hours_enabled" bson:"quiet_hours_enabled"`
}

type UserSettingsPayload struct {
	ID                 int64                `json:"id" bson:"id"`
	UserID             int64                `json:"user_id" bson:"user_id"`
	Privacy            PrivacySettings      `json:"privacy" bson:"privacy"`
	Notifications      NotificationSettings `json:"notifications" bson:"notifications"`
	Language           int                  `json:"language" bson:"language"` // Changé de string à int
	Theme              int                  `json:"theme" bson:"theme"`
	TelemetryVector    []float32            `json:"telemetry_vector" bson:"telemetry_vector"`
	TelemetryTags      []string             `json:"telemetry_tags" bson:"telemetry_tags"`
	TelemetryTimestamp int64                `json:"telemetry_timestamp" bson:"telemetry_timestamp"`
	CreatedAt          time.Time            `json:"created_at" bson:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at" bson:"updated_at"`
}
