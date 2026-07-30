package user_settings_models

// UpdateNotificationsInput utilise des pointeurs pour chaque booléen afin de gérer proprement la valeur 'false'
type UpdateNotificationsInput struct {
	MasterPushEnabled   *bool `json:"master_push_enabled" binding:"omitempty"`
	MasterEmailEnabled  *bool `json:"master_email_enabled" binding:"omitempty"`
	NotifyNewFollower   *bool `json:"notify_new_follower" binding:"omitempty"`
	NotifyFriendRequest *bool `json:"notify_friend_request" binding:"omitempty"`
	NotifyMessages      *bool `json:"notify_messages" binding:"omitempty"`
	NotifyLikes         *bool `json:"notify_likes" binding:"omitempty"`
	NotifyComments      *bool `json:"notify_comments" binding:"omitempty"`
	NotifyMentions      *bool `json:"notify_mentions" binding:"omitempty"`
	QuietHoursEnabled   *bool `json:"quiet_hours_enabled" binding:"omitempty"`
}
