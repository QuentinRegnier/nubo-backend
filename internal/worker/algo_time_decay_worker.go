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
	ViewCount    int // NOUVEAU : Indispensable pour ne pas perdre les vues au recalcul
	HasMedia     bool
	CreatedAt    time.Time
	Hashtags     []string
	Visibility   int
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
					// TODO rendre dynamique isFlagged
					service.UpdateScoreWithMetrics(
						ctx,
						job.PostID,
						job.LikeCount,
						job.CommentCount,
						job.ViewCount, // NOUVEAU : Transmission du view_count
						mediaCount,
						job.CreatedAt,
						job.Hashtags,
						job.Visibility,
						0,
					)
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

	// CORRECTION SQL : Ajout de view_count et visibility dans le SELECT
	query := `
		SELECT id, like_count, comment_count, view_count, has_media, created_at, hashtags, visibility
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
				// CORRECTION DU SCAN : On map toutes les colonnes dans l'ordre du SELECT
				err := rows.Scan(
					&job.PostID,
					&job.LikeCount,
					&job.CommentCount,
					&job.ViewCount,
					&job.HasMedia,
					&job.CreatedAt,
					pq.Array(&job.Hashtags),
					&job.Visibility,
				)
				if err != nil {
					log.Printf("⚠️ Erreur de scan dans runTierCron: %v", err)
					continue
				}
				jobs <- job
			}
			if err := rows.Close(); err != nil {
				log.Printf("⚠️ Erreur fermeture rows Cron Tier (%s-%s) : %v", minAge, maxAge, err)
			}
		}
	}
}
