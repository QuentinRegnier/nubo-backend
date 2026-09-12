package conversation_models

// UpdateMemberSettingsInput valide la mise à jour des paramètres personnels d'une conversation.
type UpdateMemberSettingsInput struct {
	ConversationID    int64 `json:"conversation_id" binding:"required"`
	IsMuted           bool  `json:"is_muted" binding:"omitempty"`
	MuteExpiresAt     int64 `json:"mute_expires_at" binding:"omitempty"`
	MediaAutoDownload bool  `json:"media_auto_download" binding:"omitempty"`
}
