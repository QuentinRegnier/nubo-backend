package user_settings_models

// UpdateNotificationsInput (Remplacement intégral PUT)
type UpdateNotificationsInput struct {
	MasterPushEnabled   bool `json:"master_push_enabled"`
	MasterEmailEnabled  bool `json:"master_email_enabled"`
	NotifyLikes         bool `json:"notify_likes"`
	NotifyComments      bool `json:"notify_comments"`
	NotifyMentions      bool `json:"notify_mentions"`
	NotifyNewFollower   bool `json:"notify_new_follower"`
	NotifyFriendRequest bool `json:"notify_friend_request"`
	NotifyMessages      bool `json:"notify_messages"`
	NotifyGroupInvites  bool `json:"notify_group_invites"` // NOUVEAU
}
