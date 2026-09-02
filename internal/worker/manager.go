package worker

import (
	"context"
	"sync"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// StartBackgroundWorkers lance les 64 ouvriers qui tournent H24 pour vider les Queues.
// Plus besoin de Sentinel ici, Redis gère la RAM via volatile-lfu.
func StartBackgroundWorkers(ctx context.Context) {
	logger.Log.Info().Msg("Démarrage du moteur de persistance (64 Workers)...")

	var wg sync.WaitGroup

	// Lancement du Worker Pool de Recalcul des Scores (Time-Decay)
	StartScoreUpdaterCron(ctx)

	// Lancement du nettoyeur de Tags (Canonicalisation)
	StartHashtagCanonCron(ctx)

	// Lancement du calcul des Tendances de Hashtags (15m)
	StartHashtagTrendCron(ctx)

	// Lancement du Moteur de Warm-up Algorithmique (Génération asynchrone des flux)
	StartFeedWarmupCron(ctx)

	// NOUVEAU : Lancement du Garbage Collector de Médias
	StartMediaCleanupCron(ctx)

	// === NOUVEAU : Lancement du Worker de Push Notifications ===
	StartPushNotificationWorker(ctx)

	// On lance 64 goroutines (une par shard Redis)
	for i := 0; i < redis.QueueShards; i++ {
		wg.Add(1)
		go func(shardID int) {
			defer wg.Done()
			runWorker(ctx, shardID)
		}(i)
	}

	// On n'attend pas ici, le main s'en charge.
	logger.Log.Info().Msg("Moteur de persistance asynchrone opérationnel.")
}
