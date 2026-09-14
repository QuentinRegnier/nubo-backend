package user_settings_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
)

// UpdateDisplay fusionne les réglages esthétiques et de contenu puis délègue la sauvegarde au Write-Behind
func UpdateDisplay(ctx context.Context, userID int64, input user_settings_models.UpdateDisplayInput) (user_settings_models.UpdateDisplayOutput, error) {
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if err != nil || settings.ID == 0 {
		return user_settings_models.UpdateDisplayOutput{}, nubo_error.NewNotFound("SETTINGS_NOT_FOUND", "Paramètres de l'utilisateur introuvables.", err)
	}

	// Mise à jour de la nouvelle struct "DisplayAndContent"
	settings.DisplayAndContent.Language = input.Language
	settings.DisplayAndContent.Theme = input.Theme
	settings.DisplayAndContent.SafeForCommute = input.SafeForCommute
	settings.UpdatedAt = time.Now().UTC()

	// Sauvegarde L1
	if err := object_cache_service.SetUserSettings(ctx, settings); err != nil {
		return user_settings_models.UpdateDisplayOutput{}, err
	}

	// Notification Websocket
	_ = realtime_service.DistributeToUsers(ctx, "user.settings_updated", settings, []int64{userID})

	// Persistance (Write-Behind)
	return user_settings_models.UpdateDisplayOutput{
		UserSettingsUpdateAt: settings.UpdatedAt,
	}, redis.EnqueueDB(ctx, settings.ID, userID, redis.EntityUserSettings, redis.ActionUpdate, settings, redis.TargetAll)
}
