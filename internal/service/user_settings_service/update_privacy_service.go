package user_settings_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
)

// UpdatePrivacy modifie les paramètres de confidentialité et délègue la sauvegarde au Write-Behind
func UpdatePrivacy(ctx context.Context, userID int64, input user_settings_models.UpdatePrivacyInput) error {
	// 1. Récupération des paramètres actuels (Cascade L1 -> L2 -> L3 garantie par ce service)
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if err != nil || settings.ID == 0 {
		return nubo_error.NewNotFound("SETTINGS_NOT_FOUND", "Paramètres de l'utilisateur introuvables.", err)
	}

	// 2. Remplacement intégral (Méthode PUT)
	settings.Privacy.ProfileVisibility = input.ProfileVisibility
	settings.Privacy.PostVisibilityDefault = input.PostVisibilityDefault
	settings.Privacy.ConversationPermission = input.ConversationPermission
	settings.Privacy.AddGroupPermission = input.AddGroupPermission
	settings.Privacy.AllowTagging = input.AllowTagging
	settings.Privacy.AllowMentions = input.AllowMentions
	settings.Privacy.ShowOnlineStatus = input.ShowOnlineStatus
	settings.Privacy.ShowLocation = input.ShowLocation
	settings.Privacy.SearchByEmailPhone = input.SearchByEmailPhone
	settings.Privacy.AllowContentSharing = input.AllowContentSharing

	settings.UpdatedAt = time.Now().UTC()

	// 3. Mise à jour immédiate du Cache L1
	if err := object_cache_service.SetUserSettings(ctx, settings); err != nil {
		return err
	}

	// === NOUVEAU : MISE À JOUR SYNCHRONE DU SPEED CACHE ===
	_ = cache_service.UpdateUserSpeedCachePrivacy(
		ctx,
		settings.UserID,
		settings.Privacy.ConversationPermission,
		settings.Privacy.AddGroupPermission,
	)

	// 4. Envoi notification
	_ = realtime_service.DistributeToUsers(ctx, "user.settings_updated", settings, []int64{userID})

	// 5. Persistance Asynchrone (Write-Behind vers Mongo et Postgres avec l'objet complet)
	return redis.EnqueueDB(ctx, settings.ID, userID, redis.EntityUserSettings, redis.ActionUpdate, settings, redis.TargetAll)
}
