package telemetry_service

import (
	"context"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/telemetry_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ProcessSync orchestre la synchronisation bidirectionnelle et l'envoi de la télémétrie aux workers.
func ProcessSync(ctx context.Context, input telemetry_models.SyncInput) (telemetry_models.SyncOutput, error) {
	output := telemetry_models.SyncOutput{
		NeedUpdate: false,
		Status:     "accepted",
	}

	// 1. EXTRACTION DES TIMESTAMPS
	clientTimestamp := input.Payload.Meta.ClientUpdatedAt
	serverTimestamp, _ := cache_service.GetTelemetryTimestamp(ctx, input.UserID)

	// 2. RÉSOLUTION DES CONFLITS (Vecteur et Tags)
	if clientTimestamp >= serverTimestamp {
		// -> LE CLIENT A RAISON (On sauvegarde en RAM)

		_ = cache_service.SetTelemetryTimestamp(ctx, input.UserID, clientTimestamp)

		if len(input.Payload.Vector.Values) == variables.VectorDimTotal {
			oldValues, err := cache_service.GetTelemetryVector(ctx, input.UserID)
			if err == nil {
				algorithm_service.InvalidatePersonalizedFeedCache(ctx, input.UserID, oldValues, input.Payload.Vector.Values)
			}
			_ = cache_service.SetTelemetryVector(ctx, input.UserID, input.Payload.Vector.Values)
		}

		if len(input.Payload.TopTags) > 0 {
			_ = cache_service.SetTelemetryTags(ctx, input.UserID, input.Payload.TopTags)
		}

		// -------------------------------------------------------------------------
		// NOUVEAU : PERSISTANCE ASYNCHRONE DU VECTEUR (Edge-to-Cloud Backup)
		// -------------------------------------------------------------------------
		// On charge l'objet complet pour ne pas écraser les autres paramètres (thème, privacy)
		// /!\ Utilise la fonction exacte de ton architecture pour charger les UserSettings depuis le cache L1
		settings, err := object_cache_service.GetUserSettingsCascade(ctx, input.UserID)
		if err == nil && settings.ID != 0 {
			// Mise à jour de l'ADN algorithmique sur l'objet
			settings.TelemetryVector = input.Payload.Vector.Values
			settings.TelemetryTags = input.Payload.TopTags
			settings.TelemetryTimestamp = clientTimestamp
			settings.UpdatedAt = time.Now().UTC()

			// 1. On remet l'objet complet à jour dans l'Object Cache (L1)
			_ = object_cache_service.SetUserSettings(ctx, settings)

			// 2. On délègue la sauvegarde BDD aux workers
			// partitionKey = input.UserID pour atterrir dans le bon shard et garantir l'ordre
			err = redis.EnqueueDB(ctx, settings.ID, input.UserID, redis.EntityUserSettings, redis.ActionUpdate, settings, redis.TargetAll)
			if err != nil {
				log.Printf("  CRITICAL: Échec EnqueueDB pour la sauvegarde du vecteur utilisateur %d: %v", input.UserID, err)
			}
		} else {
			log.Printf("  WARNING: Impossible de charger les UserSettings pour sauvegarder le vecteur de l'utilisateur %d", input.UserID)
		}

	} else {
		// -> LE SERVEUR A RAISON (Le client doit se mettre à jour)
		output.NeedUpdate = true
		output.Status = "server_newer"
		output.ServerUpdated = serverTimestamp

		if serverVector, err := cache_service.GetTelemetryVector(ctx, input.UserID); err == nil {
			output.ServerVector = serverVector
		}
		if serverTags, err := cache_service.GetTelemetryTags(ctx, input.UserID); err == nil {
			output.ServerTags = serverTags
		}
	}

	// 3. THUNDERING HERD PROTECTOR : ENVOI ASYNCHRONE DE LA TÉLÉMÉTRIE
	for _, event := range input.Payload.Telemetry {
		if event.DwellTimeMs < 500 && !event.IsClicked && !event.ProfileVisit && !event.DeepScroll {
			continue
		}

		// Envoi à la file d'attente (Les workers mettront à jour les métriques du post)
		_ = redis.EnqueueDB(ctx, event.PostID, input.UserID, redis.EntityTelemetry, redis.ActionCreate, event, redis.TargetAll)
	}

	return output, nil
}
