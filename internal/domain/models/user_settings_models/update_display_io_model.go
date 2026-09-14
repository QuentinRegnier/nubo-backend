package user_settings_models

import "time"

// UpdateDisplayInput valide les modifications d'affichage et de contenu
type UpdateDisplayInput struct {
	Theme          int  `json:"theme" binding:"omitempty,oneof=0 1 2"`
	Language       int  `json:"language" binding:"omitempty,min=0"`
	SafeForCommute bool `json:"safe_for_commute" binding:"omitempty"` // NOUVEAU
}

type UpdateDisplayOutput struct {
	UserSettingsUpdateAt time.Time `json:"user_settings_update_at"`
}
