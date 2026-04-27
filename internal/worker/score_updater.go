package worker

import (
	"context"
	"log"
	"runtime"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
)

// StartScoreUpdaterCron initialise le Worker Pool basé sur le nombre de threads CPU
// et lance les tickers étagés pour actualiser le Time-Decay de l'algorithme.
func StartScoreUpdaterCron(ctx context.Context) {
	// File d'attente (Buffer) des posts à recalculer
	jobs := make(chan int64, 10000)

	// Limite de concurrence matérielle stricte
	numWorkers := runtime.GOMAXPROCS(0)
	log.Printf("⏱️ Démarrage du Time-Decay Engine avec %d Workers CPU...", numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case postID := <-jobs:
					// On appelle la fonction de service qui fait les maths et met à jour Redis
					service.UpdatePostRecommendationScore(ctx, postID, nil)
				}
			}
		}()
	}

	// Tiers de rafraîchissement
	// Tier 1 : < 6h -> Toutes les 2 min
	go runTierCron(ctx, jobs, 2*time.Minute, "0", "6 hours")
	// Tier 2 : 6h - 24h -> Toutes les 15 min
	go runTierCron(ctx, jobs, 15*time.Minute, "6 hours", "24 hours")
	// Tier 3 : 24h - 72h -> Toutes les 60 min
	go runTierCron(ctx, jobs, 60*time.Minute, "24 hours", "72 hours")
	// Tier 4 : > 72h -> Toutes les 6 heures (Limité à 30 jours pour préserver la DB)
	go runTierCron(ctx, jobs, 6*time.Hour, "72 hours", "30 days")
}

func runTierCron(ctx context.Context, jobs chan<- int64, interval time.Duration, minAge, maxAge string) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Requête SQL optimisée : on ne sélectionne que l'ID des posts visibles
	query := `
		SELECT id FROM content.posts 
		WHERE created_at <= NOW() - $1::interval 
		AND created_at > NOW() - $2::interval 
		AND visibility != 2
	`

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := postgres.PostgresDB.QueryContext(ctx, query, minAge, maxAge)
			if err != nil {
				log.Printf("⚠️ Erreur Cron Tier (%s-%s) : %v", minAge, maxAge, err)
				continue
			}

			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err == nil {
					// Pousse le travail dans le channel pour les workers CPU
					jobs <- id
				}
			}
			if err := rows.Close(); err != nil {
				log.Printf("⚠️ Erreur fermeture rows Cron Tier (%s-%s) : %v", minAge, maxAge, err)
			}
		}
	}
}
