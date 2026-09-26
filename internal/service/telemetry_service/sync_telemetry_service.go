package telemetry_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/telemetry_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : SYNCHRONISATION DE LA TÉLÉMÉTRIE (EDGE TO CLOUD)
// ############################################################################

// ProcessSyncTelemetry orchestre la synchronisation bidirectionnelle du vecteur
// comportemental et l'envoi asynchrone des Analytics aux Workers.
func ProcessSyncTelemetry(ctx context.Context, input telemetry_models.SyncTelemetryInput) (telemetry_models.SyncTelemetryOutput, error) {

	syncOutput := telemetry_models.SyncTelemetryOutput{
		NeedUpdate: false,
		Status:     "accepted",
	}

	// ── ÉTAPE 1 : EXTRACTION DES TIMESTAMPS DE VERSIONNING ──────────────────

	clientTelemetryTimestamp := input.Payload.Meta.ClientUpdatedAt
	serverTelemetryTimestamp, _ := cache_service.GetTelemetryTimestamp(ctx, input.UserID)

	// ── ÉTAPE 2 : RÉSOLUTION DES CONFLITS (VECTEUR ET TAGS LSH) ─────────────

	if clientTelemetryTimestamp >= serverTelemetryTimestamp {
		// -> LE CLIENT EST L'AUTORITÉ (On sauvegarde en RAM L1)

		_ = cache_service.SetTelemetryTimestamp(ctx, input.UserID, clientTelemetryTimestamp)

		if len(input.Payload.Vector.Values) == variables.VectorDimTotal {
			oldVectorValues, errRedis := cache_service.GetTelemetryVector(ctx, input.UserID)
			if errRedis == nil {
				// Invalidation ciblée si les clusters sémantiques changent radicalement
				algorithm_service.InvalidatePersonalizedFeedCache(ctx, input.UserID, oldVectorValues, input.Payload.Vector.Values)
			}
			_ = cache_service.SetTelemetryVector(ctx, input.UserID, input.Payload.Vector.Values)
		}

		if len(input.Payload.TopTags) > 0 {
			_ = cache_service.SetTelemetryTags(ctx, input.UserID, input.Payload.TopTags)
		}

		// SAUVEGARDE ASYNCHRONE DE L'ADN (EDGE-TO-CLOUD BACKUP)
		// On charge l'objet complet `UserSettings` pour ne pas écraser la confidentialité (L1)
		userSettingsPayload, errSettings := object_cache_service.GetUserSettingsCascade(ctx, input.UserID)

		if errSettings == nil && userSettingsPayload.ID != 0 {
			// Injection de l'ADN algorithmique sur l'objet
			userSettingsPayload.TelemetryVector = input.Payload.Vector.Values
			userSettingsPayload.TelemetryTags = input.Payload.TopTags
			userSettingsPayload.TelemetryTimestamp = clientTelemetryTimestamp
			userSettingsPayload.UpdatedAt = domain.NowMillis()

			// 1. Écrasement synchronisé de l'Object Cache LFU (L1)
			_ = object_cache_service.SetUserSettings(ctx, userSettingsPayload)

			// 2. Délégation Write-Behind via Workers (PartitionKey = UserID pour le sharding)
			errQueue := redis.EnqueueDB(ctx, userSettingsPayload.ID, input.UserID, redis.EntityUserSettings, redis.ActionUpdate, userSettingsPayload, redis.TargetAll)
			if errQueue != nil {
				logger.Log.Error().Err(errQueue).Int64("user_id", input.UserID).Msg("Échec du Write-Behind pour la sauvegarde du vecteur IA")
				// Non bloquant : la donnée est saine en RAM, le Worker tentera de survivre.
			}
		} else {
			logger.Log.Warn().Err(errSettings).Int64("user_id", input.UserID).Msg("Impossible de charger les UserSettings pour sauvegarder le vecteur")
		}

	} else {
		// -> LE SERVEUR EST L'AUTORITÉ (Le client a du retard, il doit se mettre à jour)

		syncOutput.NeedUpdate = true
		syncOutput.Status = "server_newer"
		syncOutput.ServerUpdated = serverTelemetryTimestamp

		if serverVectorData, errRedis := cache_service.GetTelemetryVector(ctx, input.UserID); errRedis == nil {
			syncOutput.ServerVector = serverVectorData
		}
		if serverTagsData, errRedis := cache_service.GetTelemetryTags(ctx, input.UserID); errRedis == nil {
			syncOutput.ServerTags = serverTagsData
		}
	}

	// ── ÉTAPE 3 : THUNDERING HERD PROTECTOR (RÉCEPTION ASYNCHRONE) ──────────

	// Traitement du batch des logs de lecture (Dwell time, Clicks, etc.)
	for _, telemetryEvent := range input.Payload.Telemetry {

		// A. Filtre Anti-Bruit (Ghost Scroll) : Si < 500ms sans interaction, on ignore
		if telemetryEvent.DwellTimeMs < variables.MaxGhostScrollTime && !telemetryEvent.IsClicked && !telemetryEvent.ProfileVisit && !telemetryEvent.DeepScroll {
			continue
		}

		// B. Détection Qualitative de la Vue (Action ou > 1.5s de lecture cognitive)
		if telemetryEvent.DwellTimeMs >= variables.MinCognitiveReadTime || telemetryEvent.IsClicked || telemetryEvent.DeepScroll || telemetryEvent.ProfileVisit {

			// MISE À JOUR SYNCHRONE DU CACHE L1 (Temps Réel)
			if targetPostPayload, errCache := object_cache_service.GetPostFromObjectCache(ctx, telemetryEvent.PostID); errCache == nil {
				targetPostPayload.ViewCount += 1
				_ = object_cache_service.SetPostInObjectCache(ctx, targetPostPayload)

				// L'algorithme réévalue l'attractivité du post pour les autres utilisateurs
				cache_service.EvaluatePostAfterView(ctx, targetPostPayload)
			}
		}

		// C. Envoi à la file d'attente (Write-Behind SQL)
		// Les Workers mettront à jour `telemetry_dwell_sum`, `view_count`, etc.
		errTelemetryQueue := redis.EnqueueDB(ctx, telemetryEvent.PostID, input.UserID, redis.EntityTelemetry, redis.ActionCreate, telemetryEvent, redis.TargetAll)
		if errTelemetryQueue != nil {
			logger.Log.Error().Err(errTelemetryQueue).Int64("post_id", telemetryEvent.PostID).Msg("Échec du Write-Behind pour un événement de télémétrie")
			return syncOutput, nubo_error.NewInternal()
		}
	}

	return syncOutput, nil
}
