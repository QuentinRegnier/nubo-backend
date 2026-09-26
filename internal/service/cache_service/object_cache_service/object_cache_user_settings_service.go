package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : OBJECT CACHE (PARAMÈTRES UTILISATEUR LFU)
// ############################################################################

// GetUserSettings lit uniquement depuis la RAM L1 (O(1)).
func GetUserSettings(ctx context.Context, userID int64) (user_settings_models.UserSettingsPayload, error) {
	var userSettingsPayload user_settings_models.UserSettingsPayload

	// On utilise l'ID de l'utilisateur comme clé car c'est une relation 1-to-1 stricte
	errRedis := redis.UserSettings.GetObject(ctx, userID, &userSettingsPayload)
	if errRedis != nil {
		return user_settings_models.UserSettingsPayload{}, errRedis
	}

	return userSettingsPayload, nil
}

// SetUserSettings sauvegarde l'objet de paramètres en RAM L1.
func SetUserSettings(ctx context.Context, userSettingsPayload user_settings_models.UserSettingsPayload) error {
	errRedis := redis.UserSettings.SetObject(ctx, userSettingsPayload.UserID, userSettingsPayload)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("user_id", userSettingsPayload.UserID).Msg("Impossible de sauvegarder les UserSettings en RAM L1")
		return nubo_error.NewInternal()
	}
	return nil
}

// GetUserSettingsCascade tente le L1, puis le L2 (Mongo), puis le L3 (Postgres), et réhydrate la RAM.
func GetUserSettingsCascade(ctx context.Context, userID int64) (user_settings_models.UserSettingsPayload, error) {

	// ── ÉTAPE 1 : TENTATIVE L1 (REDIS RAM) ──────────────────────────────────
	cachedSettings, errL1 := GetUserSettings(ctx, userID)
	if errL1 == nil && cachedSettings.ID != 0 {
		return cachedSettings, nil
	}

	// ── ÉTAPE 2 : TENTATIVE L2 (MONGODB WARM STORAGE) ────────────────────────
	var mongoSettingsPayload user_settings_models.UserSettingsPayload
	mongoDocumentsList, errMongo := mongo.UserSettings.Get(map[string]any{"user_id": userID}, nil)

	if errMongo == nil && len(mongoDocumentsList) > 0 {
		if errStruct := pkg.ToStruct(mongoDocumentsList[0], &mongoSettingsPayload); errStruct == nil {
			_ = SetUserSettings(ctx, mongoSettingsPayload) // Réhydratation L1 silencieuse
			return mongoSettingsPayload, nil
		}
	}

	// ── ÉTAPE 3 : TENTATIVE L3 (POSTGRESQL COLD STORAGE) ────────────────────
	postgresSettings, errPg := postgres.FuncLoadUserSettings(ctx, userID)
	if errPg != nil {
		logger.Log.Error().Err(errPg).Int64("user_id", userID).Msg("Erreur critique L3 lors du chargement des UserSettings")
		return user_settings_models.UserSettingsPayload{}, nubo_error.NewInternal()
	}

	if postgresSettings.ID != 0 {
		// A. Réhydratation L1 (Redis - Immédiate en RAM)
		_ = SetUserSettings(ctx, postgresSettings)

		// B. Réhydratation L2 (Mongo - Asynchrone via les workers de la file d'attente)
		go func(payloadToSync user_settings_models.UserSettingsPayload) {
			backgroundCtx := context.Background() // Contexte détaché de la requête HTTP
			_ = redis.EnqueueDB(backgroundCtx, payloadToSync.ID, payloadToSync.UserID, redis.EntityUserSettings, redis.ActionUpdate, payloadToSync, redis.TargetMongo)
		}(postgresSettings)

		return postgresSettings, nil
	}

	return user_settings_models.UserSettingsPayload{}, nubo_error.NewNotFound(nubo_error.CodeNotFound, "Paramètres utilisateur introuvables.", nil)
}
