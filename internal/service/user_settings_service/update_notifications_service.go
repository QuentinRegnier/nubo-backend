package user_settings_service

import (
	"context"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// UpdateNotifications fusionne les nouveaux réglages de notifications et délègue la sauvegarde au Write-Behind
func UpdateNotifications(ctx context.Context, userID int64, input user_settings_models.UpdateNotificationsInput) error {
	// 1. Récupération des paramètres actuels (Cascade L1 -> L2 -> L3)
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if err != nil || settings.ID == 0 {
		return errors.New("paramètres de l'utilisateur introuvables")
	}

	// 2. Fusion des champs (on écrase uniquement si le pointeur n'est pas nul)
	if input.MasterPushEnabled != nil {
		settings.Notifications.MasterPushEnabled = *input.MasterPushEnabled
	}
	if input.MasterEmailEnabled != nil {
		settings.Notifications.MasterEmailEnabled = *input.MasterEmailEnabled
	}
	if input.NotifyNewFollower != nil {
		settings.Notifications.NotifyNewFollower = *input.NotifyNewFollower
	}
	if input.NotifyFriendRequest != nil {
		settings.Notifications.NotifyFriendRequest = *input.NotifyFriendRequest
	}
	if input.NotifyMessages != nil {
		settings.Notifications.NotifyMessages = *input.NotifyMessages
	}
	if input.NotifyLikes != nil {
		settings.Notifications.NotifyLikes = *input.NotifyLikes
	}
	if input.NotifyComments != nil {
		settings.Notifications.NotifyComments = *input.NotifyComments
	}
	if input.NotifyMentions != nil {
		settings.Notifications.NotifyMentions = *input.NotifyMentions
	}
	if input.QuietHoursEnabled != nil {
		settings.Notifications.QuietHoursEnabled = *input.QuietHoursEnabled
	}

	settings.UpdatedAt = time.Now().UTC()

	// 3. Mise à jour immédiate du Cache L1
	if err := object_cache_service.SetUserSettings(ctx, settings); err != nil {
		return err
	}

	// 4. Persistance Asynchrone (Write-Behind)
	return redis.EnqueueDB(ctx, settings.ID, userID, redis.EntityUserSettings, redis.ActionUpdate, settings, redis.TargetAll)
}
