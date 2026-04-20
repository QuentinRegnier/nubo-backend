package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
)

// --- CONFIGURATION DU CERVEAU ---
const (
	CriticalDelay       = 2 * time.Second
	HighVolumeThreshold = 2000
	MaxBatchSize        = 5000
)

func runWorker(ctx context.Context, shardID int) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			stats, err := redis.GetShardStats(ctx, shardID)
			if err != nil {
				log.Printf("⚠️ Worker %d: Impossible de lire les stats: %v", shardID, err)
				continue
			}

			if len(stats) == 0 {
				continue
			}

			selectedStats := decideNextBatch(stats)
			if selectedStats == nil {
				continue
			}

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
				processBatch(ctx, events)
			}
		}
	}
}

func decideNextBatch(stats []redis.QueueStats) *redis.QueueStats {
	var bestCandidate *redis.QueueStats
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

	if bestCandidate != nil {
		return bestCandidate
	}

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
		return bestCandidate
	}

	for i := range stats {
		s := &stats[i]
		if bestCandidate == nil || s.Delay > bestCandidate.Delay {
			bestCandidate = s
		}
	}

	return bestCandidate
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

	// Exécution Parallèle : Mongo, Postgres ET Mise à jour du MOST Cache
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

	go func() {
		// Mise à jour de l'index de découverte (MOST Cache)
		updateMostCache(ctx, events)
		done <- true
	}()

	// On attend que les 3 Goroutines aient terminé
	<-done
	<-done
	<-done
}

// updateMostCache intercepte les événements pour alimenter les ZSETs (Tags, Profils, Classements)
func updateMostCache(ctx context.Context, events []redis.AsyncEvent) {
	for _, e := range events {

		// 1. SI C'EST UN NOUVEAU POST
		if e.Type == redis.EntityPost && e.Action == redis.ActionCreate {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post domain.PostRequest
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					// A. Algorithme de Recommandation (Tags, Global, Recent)
					service.UpdatePostRecommendationScore(ctx, post.ID, post.Hashtags)
					// B. Chronologie Utilisateur (Grille Profil)
					service.AddPostToUserProfile(ctx, post.UserID, post.ID)
				}
			}
		}

		// 2. SI C'EST UN NOUVEAU LIKE
		if e.Type == redis.EntityLike && e.Action == redis.ActionCreate {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				// Structure temporaire pour récupérer l'ID du post liké
				var like struct {
					TargetID int64 `json:"target_id"`
				}
				if err := json.Unmarshal(jsonBytes, &like); err == nil && like.TargetID != 0 {
					// A. Compétition pure : on augmente le compteur de likes absolu
					service.IncrementPostMetric(ctx, like.TargetID, "likes")

					// B. Algorithme : Le post a reçu un like, on RECALCULE son score de recommandation globale !
					// Note : On passe "nil" pour les hashtags car on ne les a pas dans l'événement de Like.
					// Cela mettra à jour le classement Global et Recent, mais pas les Tags (pour le moment).
					service.UpdatePostRecommendationScore(ctx, like.TargetID, nil)
				}
			}
		}

		// 3. (Optionnel pour le futur) SI C'EST UNE NOUVELLE VUE
		// if e.Type == redis.EntityView && e.Action == redis.ActionCreate { ... }
	}
}
