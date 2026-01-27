package worker

import (
	"context"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// --- CONFIGURATION DU CERVEAU ---
const (
	// Si une requête attend depuis plus de 2 secondes, c'est l'alerte rouge.
	CriticalDelay = 2 * time.Second

	// Si on a plus de 2000 items d'un coup, c'est très rentable d'envoyer.
	HighVolumeThreshold = 2000

	// Taille maximale d'un batch (pour ne pas exploser la RAM du worker)
	MaxBatchSize = 5000
)

func runWorker(ctx context.Context, shardID int) {
	// Petit ticker pour ne pas spammer Redis si tout est vide (poll toutes les 50ms)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return // Arrêt propre

		case <-ticker.C:
			// 1. ANALYSE : On récupère le Dashboard du shard (0.5ms)
			stats, err := redis.GetShardStats(ctx, shardID)
			if err != nil {
				log.Printf("⚠️ Worker %d: Impossible de lire les stats: %v", shardID, err)
				continue
			}

			if len(stats) == 0 {
				continue // Rien à faire, on attend le prochain tick
			}

			// 2. DÉCISION : Quel groupe traiter en priorité ?
			selectedStats := decideNextBatch(stats)

			if selectedStats == nil {
				continue
			}

			// 3. ACTION : On récupère le batch ciblé
			// On limite la taille à MaxBatchSize
			batchSize := selectedStats.Count
			if batchSize > MaxBatchSize {
				batchSize = MaxBatchSize
			}

			events, err := redis.PopSmartBatch(
				ctx,
				shardID,
				selectedStats.Type,
				selectedStats.Action,
				batchSize,
			)

			if err != nil {
				log.Printf("⚠️ Worker %d: Erreur PopSmartBatch: %v", shardID, err)
				continue
			}

			if len(events) > 0 {
				// 4. TRAITEMENT : On envoie aux BDD
				// (Fonction processBatch inchangée, elle s'occupe juste d'appeler les flushers)
				processBatch(ctx, events)
			}
		}
	}
}

// decideNextBatch contient l'intelligence artificielle de tri
// Retourne un pointeur vers la ligne de stats gagnante
func decideNextBatch(stats []redis.QueueStats) *redis.QueueStats {
	var bestCandidate *redis.QueueStats

	// --- RÈGLE 1 : URGENCE ABSOLUE (Retard > 2s) ---
	// On cherche celui qui a le plus grand retard critique
	var maxDelay time.Duration

	for i := range stats {
		s := &stats[i]
		if s.Delay >= CriticalDelay {
			if s.Delay > maxDelay {
				maxDelay = s.Delay
				bestCandidate = s
			}
		}
	}

	// Si on a trouvé une urgence, on la traite tout de suite !
	if bestCandidate != nil {
		// log.Printf("🔥 URGENCE : %s %s est en retard de %v", bestCandidate.Type, bestCandidate.Action, bestCandidate.Delay)
		return bestCandidate
	}

	// --- RÈGLE 2 : RENTABILITÉ (Volume > 2000) ---
	// Sinon, on cherche celui qui a le plus gros volume
	var maxCount int64

	for i := range stats {
		s := &stats[i]
		if s.Count >= HighVolumeThreshold {
			if s.Count > maxCount {
				maxCount = s.Count
				bestCandidate = s
			}
		}
	}

	if bestCandidate != nil {
		// log.Printf("📦 VOLUME : %s %s a %d éléments", bestCandidate.Type, bestCandidate.Action, bestCandidate.Count)
		return bestCandidate
	}

	// --- RÈGLE 3 : LE RESTE (Bouche-trou) ---
	// Si personne n'est en retard et personne n'est énorme,
	// on prend simplement celui qui a le plus d'éléments pour avancer le travail.
	// (Ou celui qui est le plus vieux, au choix. Ici je privilégie le plus vieux pour éviter la famine)

	for i := range stats {
		s := &stats[i]
		if bestCandidate == nil || s.Delay > bestCandidate.Delay {
			bestCandidate = s
		}
	}

	return bestCandidate
}

// processBatch trie les événements et les envoie aux bases
func processBatch(ctx context.Context, events []redis.AsyncEvent) {
	// On sépare les tâches pour Mongo et Postgres
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

	// 3. Exécution Parallèle (Mongo et Postgres en même temps)
	// On n'attend pas que Mongo finisse pour commencer Postgres
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

	// On attend que les deux aient fini avant de prendre le prochain paquet
	<-done
	<-done
}
