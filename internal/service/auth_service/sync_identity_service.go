package auth_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// SyncIdentity synchronise uniquement l'identité (Profil et Settings) pour le Cold Start de l'App.
func SyncIdentity(ctx context.Context, input auth_models.SyncIdentityInput) (auth_models.SyncIdentityOutput, error) {
	output := auth_models.SyncIdentityOutput{
		ProfileUpdated:  false,
		SettingsUpdated: false,
	}

	// =====================================================================
	// 1. DELTA SYNC : PROFIL UTILISATEUR
	// =====================================================================
	user, errUser := mongo.MongoLoadUser(input.UserID, "", "", "")
	if errUser != nil || user.ID == 0 {
		user, _ = postgres.FuncLoadUser(input.UserID, "", "", "")
	}

	if user.ID != 0 {
		if user.UpdatedAt.UnixMilli() > input.ProfileUpdatedAt {
			output.ProfileUpdated = true
			output.Profile = user

			// === NOUVEAU : HYDRATATION DE L'AVATAR (Composition par Valeur) ===
			if user.ProfilePictureID > 0 {
				// targetID = 0 (pas lié à un post), readerID = input.UserID
				if view, err := media_service.GenerateMediaViewCascade(ctx, user.ProfilePictureID, user.ID, 0, input.UserID); err == nil {
					output.Avatar = view
				}
			}
		}
	}

	// =====================================================================
	// 2. DELTA SYNC : PARAMÈTRES
	// =====================================================================
	settings, errSettings := object_cache_service.GetUserSettingsCascade(ctx, input.UserID)
	if errSettings == nil && settings.ID != 0 {
		if settings.UpdatedAt.UnixMilli() > input.SettingsUpdatedAt {
			output.SettingsUpdated = true
			output.Settings = settings
		}
	}

	return output, nil
}
