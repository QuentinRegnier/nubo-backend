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
// # SERVICE : MISE À JOUR DES PARAMÈTRES D'AFFICHAGE
// ############################################################################

// UpdateDisplay fusionne les réglages esthétiques et de contenu puis délègue la sauvegarde au Write-Behind.
func UpdateDisplay(ctx context.Context, userID int64, input user_settings_models.UpdateDisplayInput) (user_settings_models.UpdateDisplayOutput, error) {

	// ── ÉTAPE 1 : RÉCUPÉRATION DU PAYLOAD (CASCADE L1 -> L2 -> L3) ──────────
	userSettingsPayload, errCache := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if errCache != nil || userSettingsPayload.ID == 0 {
		return user_settings_models.UpdateDisplayOutput{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Paramètres de l'utilisateur introuvables.", errCache)
	}

	// ── ÉTAPE 2 : APPLICATION DES MODIFICATIONS ─────────────────────────────
	userSettingsPayload.DisplayAndContent.Language = input.Language
	userSettingsPayload.DisplayAndContent.Theme = input.Theme
	userSettingsPayload.DisplayAndContent.SafeForCommute = input.SafeForCommute
	userSettingsPayload.UpdatedAt = domain.NowMillis()

	// ── ÉTAPE 3 : SAUVEGARDE L1 EN RAM ──────────────────────────────────────
	if errSet := object_cache_service.SetUserSettings(ctx, userSettingsPayload); errSet != nil {
		logger.Log.Warn().Err(errSet).Int64("user_id", userID).Msg("Impossible de mettre à jour les paramètres d'affichage dans le cache L1")
	}

	// ── ÉTAPE 4 : NOTIFICATION TEMPS RÉEL (WEBSOCKET) ───────────────────────
	errBroadcast := realtime_service.DistributeToUsers(ctx, variables.NotificationSettingsUpdated, userSettingsPayload, []int64{userID})
	if errBroadcast != nil {
		logger.Log.Warn().Err(errBroadcast).Msg("Échec de la distribution WebSocket pour la mise à jour d'affichage")
	}

	// ── ÉTAPE 5 : PERSISTANCE ASYNCHRONE (WRITE-BEHIND) ─────────────────────
	errQueue := redis.EnqueueDB(ctx, userSettingsPayload.ID, userID, redis.EntityUserSettings, redis.ActionUpdate, userSettingsPayload, redis.TargetAll)
	if errQueue != nil {
		logger.Log.Error().Err(errQueue).Int64("user_id", userID).Msg("Échec du Write-Behind lors de la mise à jour de l'affichage")
		return user_settings_models.UpdateDisplayOutput{}, nubo_error.NewInternal()
	}

	return user_settings_models.UpdateDisplayOutput{
		UserSettingsUpdateAt: userSettingsPayload.UpdatedAt,
	}, nil
}
