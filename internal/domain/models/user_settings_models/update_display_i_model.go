package user_settings_models

// UpdateDisplayInput valide les modifications d'affichage et de contenu
type UpdateDisplayInput struct {
	Theme          int  `json:"theme" binding:"omitempty,oneof=0 1 2"`
	Language       int  `json:"language" binding:"omitempty,min=0"`
	SafeForCommute bool `json:"safe_for_commute" binding:"omitempty"` // NOUVEAU
}
