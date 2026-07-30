package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// GetUserSettings lit uniquement depuis la RAM (O(1))
func GetUserSettings(ctx context.Context, userID int64) (user_settings_models.UserSettingsPayload, error) {
	var settings user_settings_models.UserSettingsPayload
	// On utilise l'ID de l'utilisateur comme clé car c'est une relation 1-to-1
	err := redis.UserSettings.GetObject(ctx, userID, &settings)
	return settings, err
}

// SetUserSettings sauvegarde l'objet en RAM
func SetUserSettings(ctx context.Context, settings user_settings_models.UserSettingsPayload) error {
	return redis.UserSettings.SetObject(ctx, settings.UserID, settings)
}

// GetUserSettingsCascade tente le L1, puis le L2 (Mongo), puis le L3 (Postgres), et réhydrate la RAM.
func GetUserSettingsCascade(ctx context.Context, userID int64) (user_settings_models.UserSettingsPayload, error) {
	// 1. TENTATIVE L1 (Redis)
	if s, err := GetUserSettings(ctx, userID); err == nil && s.ID != 0 {
		return s, nil
	}

	// 2. TENTATIVE L2 (MongoDB)
	var settings user_settings_models.UserSettingsPayload
	docs, errMongo := mongo.UserSettings.Get(map[string]any{"user_id": userID}, nil)
	if errMongo == nil && len(docs) > 0 {
		if err := pkg.ToStruct(docs[0], &settings); err == nil {
			_ = SetUserSettings(ctx, settings) // Réhydratation L1
			return settings, nil
		}
	}

	// 3. TENTATIVE L3 (PostgreSQL)
	sPg, errPg := postgres.FuncLoadUserSettings(ctx, userID)
	if errPg == nil && sPg.ID != 0 {
		// A. Réhydratation L2 (Mongo)
		doc, _ := pkg.ToMap(sPg)
		if doc != nil {
			_ = mongo.UserSettings.Set(doc)
		}
		// B. Réhydratation L1 (Redis)
		_ = SetUserSettings(ctx, sPg)

		return sPg, nil
	}

	return user_settings_models.UserSettingsPayload{}, nil
}
