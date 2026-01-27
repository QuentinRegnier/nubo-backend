package worker

import (
	"context"
	"log"
	"sync"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// StartBackgroundWorkers lance les 64 ouvriers qui tournent H24.
func StartBackgroundWorkers(ctx context.Context) {
	log.Println("🚜 Démarrage du moteur de persistance (64 Workers)...")

	var wg sync.WaitGroup

	// On lance 64 goroutines (une par shard Redis)
	for i := 0; i < redis.QueueShards; i++ {
		wg.Add(1)
		go func(shardID int) {
			defer wg.Done()
			runWorker(ctx, shardID)
		}(i)
	}

	// On n'attend pas ici (c'est non bloquant pour le main),
	// sauf si tu veux gérer un "Graceful Shutdown" propre.
	log.Println("✅ Moteur de persistance opérationnel.")
}
