package user_settings_service

import (
	"context"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// UpdateStyle fusionne les réglages esthétiques et linguistiques puis délègue la sauvegarde au Write-Behind
func UpdateStyle(ctx context.Context, userID int64, input user_settings_models.UpdateStyleInput) error {
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if err != nil || settings.ID == 0 {
		return errors.New("paramètres de l'utilisateur introuvables")
	}

	if input.Language != nil {
		settings.Language = *input.Language
	}
	if input.Theme != nil {
		settings.Theme = *input.Theme
	}

	settings.UpdatedAt = time.Now().UTC()

	if err := object_cache_service.SetUserSettings(ctx, settings); err != nil {
		return err
	}

	return redis.EnqueueDB(ctx, settings.ID, userID, redis.EntityUserSettings, redis.ActionUpdate, settings, redis.TargetAll)
}
