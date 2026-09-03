package worker

import (
	"context"
	"runtime"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// ScoreJob contient les métriques pré-calculées par SQL pour éviter l'hydratation N+1
type ScoreJob struct {
	PostID        int64
	LikeCount     int
	CommentCount  int
	ViewCount     int
	HasMedia      bool
	CreatedAt     time.Time
	Hashtags      []string
	Visibility    int
	PriorityLevel int // NOUVEAU
	ReportCount   int
}

// StartScoreUpdaterCron initialise le Worker Pool basé sur le nombre de threads CPU
// et lance les tickers étagés pour actualiser le Time-Decay de l'algorithme.
func StartScoreUpdaterCron(ctx context.Context) {
	// File d'attente contenant directement les métriques (Buffer 10000)
	jobs := make(chan ScoreJob, 10000)

	// Limite de concurrence matérielle stricte
	numWorkers := runtime.GOMAXPROCS(0)
	logger.Log.Info().Int("workers_cpu", numWorkers).Msg("Démarrage du Time-Decay Engine")

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
					cache_service.UpdateScoreWithMetrics(
						ctx,
						job.PostID,
						job.LikeCount,
						job.CommentCount,
						job.ViewCount,
						mediaCount,
						job.CreatedAt,
						job.Hashtags,
						job.Visibility,
						job.ReportCount,
						job.PriorityLevel, // NOUVEAU : Transmission du priority_level
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

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Appel Pur DDD via le Repository
			posts, err := postgres.FuncLoadPostsForTimeDecay(ctx, minAge, maxAge)
			if err != nil {
				logger.Log.Error().Err(err).Str("min_age", minAge).Str("max_age", maxAge).Msg("Erreur requête Time-Decay Tier")
				continue
			}

			for _, p := range posts {
				jobs <- ScoreJob{
					PostID:        p.ID,
					LikeCount:     p.LikeCount,
					CommentCount:  p.CommentCount,
					ViewCount:     p.ViewCount,
					HasMedia:      p.HasMedia,
					CreatedAt:     p.CreatedAt,
					Hashtags:      p.Hashtags,
					Visibility:    p.Visibility,
					PriorityLevel: p.PriorityLevel,
					ReportCount:   p.ReportCount,
				}
			}
		}
	}
}
