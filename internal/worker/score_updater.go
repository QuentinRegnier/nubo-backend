package worker

import (
	"context"
	"log"
	"runtime"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/lib/pq"
)

// ScoreJob contient les métriques pré-calculées par SQL pour éviter l'hydratation N+1
type ScoreJob struct {
	PostID       int64
	LikeCount    int
	CommentCount int
	HasMedia     bool
	CreatedAt    time.Time
	Hashtags     []string
}

// StartScoreUpdaterCron initialise le Worker Pool basé sur le nombre de threads CPU
// et lance les tickers étagés pour actualiser le Time-Decay de l'algorithme.
func StartScoreUpdaterCron(ctx context.Context) {
	// File d'attente contenant directement les métriques (Buffer 10000)
	jobs := make(chan ScoreJob, 10000)

	// Limite de concurrence matérielle stricte
	numWorkers := runtime.GOMAXPROCS(0)
	log.Printf("⏱️ Démarrage du Time-Decay Engine avec %d Workers CPU...", numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-jobs:
					mediaCount := 0
					if job.HasMedia {
						mediaCount = 1
					}
					// Appel du moteur mathématique pur. BDD = 0, Redis = Max
					service.UpdateScoreWithMetrics(ctx, job.PostID, job.LikeCount, job.CommentCount, mediaCount, job.CreatedAt, job.Hashtags)
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

func runTierCron(ctx context.Context, jobs chan<- ScoreJob, interval time.Duration, minAge, maxAge string) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Requête SQL enrichie : on récupère toutes les variables d'ajustement du TDD
	query := `
		SELECT id, like_count, comment_count, has_media, created_at, hashtags
		FROM content.posts 
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
				var job ScoreJob
				// Scan incluant le tableau de Strings spécifique à Postgres
				if err := rows.Scan(&job.PostID, &job.LikeCount, &job.CommentCount, &job.HasMedia, &job.CreatedAt, pq.Array(&job.Hashtags)); err == nil {
					jobs <- job
				} else {
					log.Printf("⚠️ Erreur Scan ScoreJob : %v", err)
				}
			}
			if err := rows.Close(); err != nil {
				log.Printf("⚠️ Erreur fermeture rows Cron Tier (%s-%s) : %v", minAge, maxAge, err)
			}
		}
	}
}
