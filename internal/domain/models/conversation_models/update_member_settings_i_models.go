package conversation_models

import "time"

// UpdateMemberSettingsInput valide le payload complet envoyé par l'application
type UpdateMemberSettingsInput struct {
	ConversationID    int64 `json:"conversation_id" binding:"required"`
	IsMuted           int   `json:"is_muted" binding:"min=0,max=2"`
	MuteExpiresAt     int64 `json:"mute_expires_at"`
	MediaAutoDownload bool  `json:"media_auto_download"`
}

type UpdateMemberSettingsOutput struct {
	InboxUpdateAt time.Time `json:"inbox_update_at"`
}
