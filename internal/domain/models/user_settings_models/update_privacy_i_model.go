package user_settings_models

type UpdatePrivacyInput struct {
	ProfileVisibility      int  `json:"profile_visibility" binding:"omitempty,oneof=0 1 2"`
	PostVisibilityDefault  int  `json:"post_visibility_default" binding:"omitempty,oneof=0 1 2"`
	ConversationPermission int  `json:"conversation_permission" binding:"omitempty,oneof=0 1 2"`
	AddGroupPermission     bool `json:"add_group_permission" binding:"omitempty"`
	AllowTagging           int  `json:"allow_tagging" binding:"omitempty,oneof=0 1 2"`
	AllowMentions          int  `json:"allow_mentions" binding:"omitempty,oneof=0 1 2"`
	ShowOnlineStatus       bool `json:"show_online_status" binding:"omitempty"`
	ShowLocation           bool `json:"show_location" binding:"omitempty"`
	SearchByEmailPhone     bool `json:"search_by_email_phone" binding:"omitempty"`
	AllowContentSharing    bool `json:"allow_content_sharing" binding:"omitempty"`
}
