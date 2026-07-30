package user_settings_service

import (
	"context"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// UpdatePrivacy modifie les paramètres de confidentialité et délègue la sauvegarde au Write-Behind
func UpdatePrivacy(ctx context.Context, userID int64, input user_settings_models.UpdatePrivacyInput) error {
	// 1. Récupération des paramètres actuels (Cascade L1 -> L2 -> L3 garantie par ce service)
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, userID)
	if err != nil || settings.ID == 0 {
		return errors.New("paramètres de l'utilisateur introuvables")
	}

	// 2. Fusion des champs (on écrase uniquement si le pointeur n'est pas nul)
	if input.ProfileVisibility != nil {
		settings.Privacy.ProfileVisibility = *input.ProfileVisibility
	}
	if input.PostVisibilityDefault != nil {
		settings.Privacy.PostVisibilityDefault = *input.PostVisibilityDefault
	}
	if input.ConversationPermission != nil {
		settings.Privacy.ConversationPermission = *input.ConversationPermission
	}
	if input.AllowTagging != nil {
		settings.Privacy.AllowTagging = *input.AllowTagging
	}
	if input.AllowMentions != nil {
		settings.Privacy.AllowMentions = *input.AllowMentions
	}
	if input.ShowOnlineStatus != nil {
		settings.Privacy.ShowOnlineStatus = *input.ShowOnlineStatus
	}
	if input.ShowLocation != nil {
		settings.Privacy.ShowLocation = *input.ShowLocation
	}
	if input.SearchByEmailPhone != nil {
		settings.Privacy.SearchByEmailPhone = *input.SearchByEmailPhone
	}
	if input.AllowContentSharing != nil {
		settings.Privacy.AllowContentSharing = *input.AllowContentSharing
	}

	settings.UpdatedAt = time.Now().UTC()

	// 3. Mise à jour immédiate du Cache L1
	if err := object_cache_service.SetUserSettings(ctx, settings); err != nil {
		return err
	}

	// 4. Persistance Asynchrone (Write-Behind vers Mongo et Postgres avec l'objet complet)
	return redis.EnqueueDB(ctx, settings.ID, userID, redis.EntityUserSettings, redis.ActionUpdate, settings, redis.TargetAll)
}
