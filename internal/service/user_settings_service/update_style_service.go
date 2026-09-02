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

// UpdateStyle fusionne les réglages esthétiques et linguistiques puis délègue la sauvegarde au Write-Behind
func UpdateStyle(ctx context.Context, userID int64, input user_settings_models.UpdateStyleInput) error {
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if err != nil || settings.ID == 0 {
		return nubo_error.NewNotFound("SETTINGS_NOT_FOUND", "Paramètres de l'utilisateur introuvables.", err)
	}

	settings.Language = input.Language
	settings.Theme = input.Theme

	settings.UpdatedAt = time.Now().UTC()

	if err := object_cache_service.SetUserSettings(ctx, settings); err != nil {
		return err
	}

	_ = realtime_service.DistributeToUsers(ctx, "user.settings_updated", settings, []int64{userID})

	return redis.EnqueueDB(ctx, settings.ID, userID, redis.EntityUserSettings, redis.ActionUpdate, settings, redis.TargetAll)
}
