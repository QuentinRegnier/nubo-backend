package telemetry_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/telemetry_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ProcessSyncTelemetry orchestre la synchronisation bidirectionnelle et l'envoi de la télémétrie aux workers.
func ProcessSyncTelemetry(ctx context.Context, input telemetry_models.SyncTelemetryInput) (telemetry_models.SyncTelemetryOutput, error) {
	output := telemetry_models.SyncTelemetryOutput{
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
				logger.Log.Error().Err(err).
					Int64("user_id", input.UserID).
					Msg("Échec EnqueueDB pour la sauvegarde du vecteur utilisateur")
			}
		} else {
			logger.Log.Warn().Err(err).
				Int64("user_id", input.UserID).
				Msg("Impossible de charger les UserSettings pour sauvegarder le vecteur")
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
		// Filtre strict anti-bruit (scroll fantôme)
		if event.DwellTimeMs < 500 && !event.IsClicked && !event.ProfileVisit && !event.DeepScroll {
			continue
		}

		// --- DÉTECTION QUALITATIVE DE LA VUE ---
		// Si le temps de lecture dépasse 1.5s, ou qu'il y a une action explicite, c'est une VRAIE vue.
		if event.DwellTimeMs >= 1500 || event.IsClicked || event.DeepScroll || event.ProfileVisit {
			// 1. MISE À JOUR SYNCHRONE DU L1 (TEMPS RÉEL)
			if p, err := object_cache_service.GetPostFromObjectCache(ctx, event.PostID); err == nil {
				p.ViewCount += 1
				_ = object_cache_service.SetPostInObjectCache(ctx, p)
				cache_service.EvaluatePostAfterView(ctx, p)
			}
		}

		// Envoi à la file d'attente détaillée (Les workers mettront à jour telemetry_dwell_sum, view_count, etc.)
		_ = redis.EnqueueDB(ctx, event.PostID, input.UserID, redis.EntityTelemetry, redis.ActionCreate, event, redis.TargetAll)
	}

	return output, nil
}
