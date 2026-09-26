package cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # SERVICE : GESTION CACHE DE LA TÉLÉMÉTRIE SÉMANTIQUE
// ############################################################################

// GetTelemetryVector récupère le dernier vecteur profil de l'utilisateur (ADN algorithmique).
func GetTelemetryVector(ctx context.Context, userID int64) ([]float32, error) {
	var userVector []float32
	errRedis := redis.TelemetryVectors.GetObject(ctx, userID, &userVector)

	if errRedis != nil {
		return nil, nubo_error.NewInternal()
	}

	return userVector, nil
}

// SetTelemetryVector sauvegarde le vecteur de profil (Edge-to-Cloud) avec son TTL automatique L1.
func SetTelemetryVector(ctx context.Context, userID int64, newVector []float32) error {
	errRedis := redis.TelemetryVectors.SetObject(ctx, userID, newVector)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("user_id", userID).Msg("Impossible de sauvegarder le vecteur de télémétrie")
		return nubo_error.NewInternal()
	}
	return nil
}

// SetTelemetryTags sauvegarde le Top 5 des tags de l'utilisateur (utilisé par le Magasinier LSH).
func SetTelemetryTags(ctx context.Context, userID int64, topTagsList []string) error {
	errRedis := redis.TelemetryTags.SetObject(ctx, userID, topTagsList)
	if errRedis != nil {
		logger.Log.Error().Err(errRedis).Int64("user_id", userID).Msg("Impossible de sauvegarder le Top Tags de télémétrie")
		return nubo_error.NewInternal()
	}
	return nil
}

// GetTelemetryTags récupère la liste des tags préférés.
func GetTelemetryTags(ctx context.Context, userID int64) ([]string, error) {
	var userTags []string
	errRedis := redis.TelemetryTags.GetObject(ctx, userID, &userTags)

	if errRedis != nil {
		return nil, nubo_error.NewInternal()
	}

	return userTags, nil
}

// SetTelemetryTimestamp sauvegarde la date de la dernière synchronisation Edge-to-Cloud réussie.
func SetTelemetryTimestamp(ctx context.Context, userID int64, currentTimestampMs int64) error {
	// Stockage MsgPack unifié via SetObject
	errRedis := redis.TelemetryTimestamps.SetObject(ctx, userID, currentTimestampMs)
	if errRedis != nil {
		logger.Log.Warn().Err(errRedis).Msg("Impossible de sauvegarder le timestamp de télémétrie")
		return nubo_error.NewInternal()
	}
	return nil
}

// GetTelemetryTimestamp récupère la date de la dernière synchronisation Edge-to-Cloud.
func GetTelemetryTimestamp(ctx context.Context, userID int64) (int64, error) {
	var cachedTimestamp int64
	errRedis := redis.TelemetryTimestamps.GetObject(ctx, userID, &cachedTimestamp)

	if errRedis != nil {
		return 0, nubo_error.NewInternal() // Différent de 0 pour déclencher le rafraîchissement
	}

	return cachedTimestamp, nil
}
