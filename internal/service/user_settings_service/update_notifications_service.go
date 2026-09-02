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

// UpdateNotifications fusionne les nouveaux réglages de notifications et délègue la sauvegarde au Write-Behind
func UpdateNotifications(ctx context.Context, userID int64, input user_settings_models.UpdateNotificationsInput) error {
	// 1. Récupération des paramètres actuels (Cascade L1 -> L2 -> L3)
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if err != nil || settings.ID == 0 {
		return nubo_error.NewNotFound("SETTINGS_NOT_FOUND", "Paramètres de l'utilisateur introuvables.", err)
	}

	// 2. Remplacement intégral des paramètres de notification
	settings.Notifications.MasterPushEnabled = input.MasterPushEnabled
	settings.Notifications.MasterEmailEnabled = input.MasterEmailEnabled
	settings.Notifications.NotifyNewFollower = input.NotifyNewFollower
	settings.Notifications.NotifyFriendRequest = input.NotifyFriendRequest
	settings.Notifications.NotifyMessages = input.NotifyMessages
	settings.Notifications.NotifyLikes = input.NotifyLikes
	settings.Notifications.NotifyComments = input.NotifyComments
	settings.Notifications.NotifyMentions = input.NotifyMentions
	settings.Notifications.QuietHoursEnabled = input.QuietHoursEnabled

	settings.UpdatedAt = time.Now().UTC()

	// 3. Mise à jour immédiate du Cache L1
	if err := object_cache_service.SetUserSettings(ctx, settings); err != nil {
		return err
	}

	// 4. Envoi notification
	_ = realtime_service.DistributeToUsers(ctx, "user.settings_updated", settings, []int64{userID})

	// 5. Persistance Asynchrone (Write-Behind)
	return redis.EnqueueDB(ctx, settings.ID, userID, redis.EntityUserSettings, redis.ActionUpdate, settings, redis.TargetAll)
}
