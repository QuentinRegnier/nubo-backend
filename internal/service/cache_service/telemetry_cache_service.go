package cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// GetTelemetryVector récupère le dernier vecteur de profil de l'utilisateur.
func GetTelemetryVector(ctx context.Context, userID int64) ([]float32, error) {
	var vector []float32
	err := redis.TelemetryVectors.GetObject(ctx, userID, &vector)
	return vector, err
}

// SetTelemetryVector sauvegarde le vecteur de profil (Edge-to-Cloud) avec son TTL automatique.
func SetTelemetryVector(ctx context.Context, userID int64, vector []float32) error {
	return redis.TelemetryVectors.SetObject(ctx, userID, vector)
}

// SetTelemetryTags sauvegarde le Top 5 des tags de l'utilisateur (utilisé par le Magasinier).
func SetTelemetryTags(ctx context.Context, userID int64, tags []string) error {
	return redis.TelemetryTags.SetObject(ctx, userID, tags)
}

// GetTelemetryTags récupère la liste des tags préférés.
func GetTelemetryTags(ctx context.Context, userID int64) ([]string, error) {
	var tags []string
	err := redis.TelemetryTags.GetObject(ctx, userID, &tags)
	return tags, err
}

// SetTelemetryTimestamp sauvegarde la date de la dernière synchronisation du vecteur local.
func SetTelemetryTimestamp(ctx context.Context, userID int64, timestamp int64) error {
	// On stocke le timestamp de manière unifiée avec le reste (MsgPack via SetObject)
	return redis.TelemetryTimestamps.SetObject(ctx, userID, timestamp)
}

// GetTelemetryTimestamp récupère la date de la dernière synchronisation réussie.
func GetTelemetryTimestamp(ctx context.Context, userID int64) (int64, error) {
	var timestamp int64
	err := redis.TelemetryTimestamps.GetObject(ctx, userID, &timestamp)
	if err != nil {
		return 0, err
	}
	return timestamp, nil
}
