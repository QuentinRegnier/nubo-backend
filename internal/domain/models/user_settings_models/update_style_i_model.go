package user_settings_models

type UpdateStyleInput struct {
	Language int `json:"language" binding:"omitempty,min=0"`
	Theme    int `json:"theme" binding:"omitempty,oneof=0 1 2 3"`
}
