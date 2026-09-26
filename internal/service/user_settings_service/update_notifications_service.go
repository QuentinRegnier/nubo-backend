package user_settings_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : MISE À JOUR DES PARAMÈTRES DE NOTIFICATION
// ############################################################################

// UpdateNotifications fusionne les nouveaux réglages de notifications et délègue la sauvegarde au Write-Behind.
func UpdateNotifications(ctx context.Context, userID int64, input user_settings_models.UpdateNotificationsInput) (user_settings_models.UpdateNotificationsOutput, error) {

	// ── ÉTAPE 1 : RÉCUPÉRATION DES PARAMÈTRES (CASCADE L1 -> L2 -> L3) ──────
	userSettingsPayload, errCache := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if errCache != nil || userSettingsPayload.ID == 0 {
		return user_settings_models.UpdateNotificationsOutput{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Paramètres de l'utilisateur introuvables.", errCache)
	}

	// ── ÉTAPE 2 : REMPLACEMENT INTÉGRAL DES PARAMÈTRES ──────────────────────
	userSettingsPayload.Notifications.MasterPushEnabled = input.MasterPushEnabled
	userSettingsPayload.Notifications.MasterEmailEnabled = input.MasterEmailEnabled
	userSettingsPayload.Notifications.NotifyLikes = input.NotifyLikes
	userSettingsPayload.Notifications.NotifyComments = input.NotifyComments
	userSettingsPayload.Notifications.NotifyMentions = input.NotifyMentions
	userSettingsPayload.Notifications.NotifyNewFollower = input.NotifyNewFollower
	userSettingsPayload.Notifications.NotifyFriendRequest = input.NotifyFriendRequest
	userSettingsPayload.Notifications.NotifyMessages = input.NotifyMessages
	userSettingsPayload.Notifications.NotifyGroupInvites = input.NotifyGroupInvites

	userSettingsPayload.UpdatedAt = domain.NowMillis()

	// ── ÉTAPE 3 : MISE À JOUR IMMÉDIATE L1 EN RAM ───────────────────────────
	if errSet := object_cache_service.SetUserSettings(ctx, userSettingsPayload); errSet != nil {
		logger.Log.Warn().Err(errSet).Int64("user_id", userID).Msg("Impossible de mettre à jour les paramètres de notifications dans le cache L1")
	}

	// ── ÉTAPE 4 : NOTIFICATION TEMPS RÉEL (WEBSOCKET) ───────────────────────
	errBroadcast := realtime_service.DistributeToUsers(ctx, variables.NotificationSettingsUpdated, userSettingsPayload, []int64{userID})
	if errBroadcast != nil {
		logger.Log.Warn().Err(errBroadcast).Msg("Échec de la distribution WebSocket pour la mise à jour des notifications")
	}

	// ── ÉTAPE 5 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────
	errQueue := redis.EnqueueDB(ctx, userSettingsPayload.ID, userID, redis.EntityUserSettings, redis.ActionUpdate, userSettingsPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("user_id", userID).Msg("Échec du Write-Behind lors de la mise à jour des notifications")
		return user_settings_models.UpdateNotificationsOutput{}, nubo_error.NewInternal()
	}

	return user_settings_models.UpdateNotificationsOutput{
		UserSettingsUpdateAt: userSettingsPayload.UpdatedAt,
	}, nil
}
