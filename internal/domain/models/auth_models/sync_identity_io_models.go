package auth_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"

type SyncIdentityInput struct {
	UserID            int64 `json:"-"`
	ProfileUpdatedAt  int64 `json:"profile_updated_at"`
	SettingsUpdatedAt int64 `json:"settings_updated_at"`
}

type SyncIdentityOutput struct {
	ProfileUpdated  bool                                     `json:"profile_updated"`
	Profile         UserPayload                              `json:"profile"`
	SettingsUpdated bool                                     `json:"settings_updated"`
	Settings        user_settings_models.UserSettingsPayload `json:"settings"`
}
