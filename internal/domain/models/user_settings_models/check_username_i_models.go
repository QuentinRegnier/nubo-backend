package user_settings_models

type CheckUsernameInput struct {
	Username string `json:"username" binding:"required,alphanum"`
}
