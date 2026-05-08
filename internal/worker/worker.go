package worker

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// --- CONFIGURATION DU CERVEAU (Modifiable via .env) ---
var (
	MaxBatchSize int64
	MinBackoff   time.Duration
	MaxBackoff   time.Duration
)

func init() {
	// 1. Taille du Batch (Ex: 5000)
	if val, err := strconv.ParseInt(os.Getenv("WORKER_MAX_BATCH_SIZE"), 10, 64); err == nil && val > 0 {
		MaxBatchSize = val
	} else {
		MaxBatchSize = 5000 // Valeur par défaut
	}

	// 2. Backoff Minimum (Période d'hyperactivité, ex: 50ms)
	if val, err := strconv.Atoi(os.Getenv("WORKER_MIN_BACKOFF_MS")); err == nil && val > 0 {
		MinBackoff = time.Duration(val) * time.Millisecond
	} else {
		MinBackoff = 50 * time.Millisecond // Valeur par défaut
	}

	// 3. Backoff Maximum (Sommeil profond, ex: 1000ms)
	if val, err := strconv.Atoi(os.Getenv("WORKER_MAX_BACKOFF_MS")); err == nil && val > 0 {
		MaxBackoff = time.Duration(val) * time.Millisecond
	} else {
		MaxBackoff = 1 * time.Second // Valeur par défaut
	}
}

func runWorker(ctx context.Context, shardID int) {
	currentBackoff := MinBackoff

	for {
		// 1. Vérification de l'arrêt gracieux du serveur
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 2. Blocage absolu (0 CPU) via BLMPOP / BLPOP
		// On limite la taille via MaxBatchSize (dynamique)
		events, err := redis.PopSmartBatchBlocking(ctx, shardID, MaxBatchSize)

		if err != nil {
			log.Printf("⚠️ Worker %d: Erreur Redis: %v", shardID, err)
			time.Sleep(1 * time.Second) // Protection anti-boucle infinie si Redis crashe
			continue
		}

		// 3. Traitement dynamique
		if len(events) > 0 {
			processBatch(ctx, events)
			// RESET DU SOMMEIL : on a trouvé du travail, on repasse à la vitesse max !
			currentBackoff = MinBackoff
		} else {
			// SLEEP : la file était vide (malgré le blocage), on s'endort doucement
			time.Sleep(currentBackoff)
			currentBackoff *= 2
			if currentBackoff > MaxBackoff {
				currentBackoff = MaxBackoff
			}
		}
	}
}

// processBatch trie les événements et les envoie aux bases ET au cache
func processBatch(ctx context.Context, events []redis.AsyncEvent) {
	var mongoEvents []redis.AsyncEvent
	var pgEvents []redis.AsyncEvent

	for _, evt := range events {
		if evt.Targets&redis.TargetMongo != 0 {
			mongoEvents = append(mongoEvents, evt)
		}
		if evt.Targets&redis.TargetPostgres != 0 {
			pgEvents = append(pgEvents, evt)
		}
	}

	// Étape 1 : Exécution Parallèle des bases de données (Mongo & Postgres)
	done := make(chan bool)

	go func() {
		if len(mongoEvents) > 0 {
			flushMongo(ctx, mongoEvents)
		}
		done <- true
	}()

	go func() {
		if len(pgEvents) > 0 {
			flushPostgres(ctx, pgEvents)
		}
		done <- true
	}()

	// On DOIT attendre que la BDD ait validé les transactions sur le disque
	// avant de mettre à jour le cache, sinon on lira des valeurs périmées.
	<-done
	<-done

	// Étape 2 : Mise à jour de l'index de découverte (MOST Cache)
	// S'exécute de manière asynchrone mais séquencée APRÈS la BDD.
	updateMostCache(ctx, events)
}
