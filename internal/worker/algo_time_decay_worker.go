package worker

import (
	"context"
	"runtime"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ScoreJob contient les métriques pré-calculées par la base de données.
// Ce format plat évite le problème de requêtes "N+1" et supprime le besoin d'hydrater
// des objets complets uniquement pour mettre à jour un score mathématique.
type ScoreJob struct {
	PostID           int64
	LikeCount        int
	CommentCount     int
	ViewCount        int
	HasMedia         bool
	CreatedAt        time.Time
	Hashtags         []string
	IndirectHashtags []string
	Visibility       int
	PriorityLevel    int
	ReportCount      int
}

// ############################################################################
// # WORKER : TIME-DECAY ENGINE (MOTEUR DE DÉCLIN TEMPOREL)
// ############################################################################

// StartScoreUpdaterCron initialise le Worker Pool basé sur le nombre de threads du CPU
// et lance les planificateurs (Tickers) étagés pour actualiser le score algorithmique des posts.
func StartScoreUpdaterCron(ctx context.Context) {
	// ── ÉTAPE 1 : INITIALISATION DE LA FILE D'ATTENTE (BUFFER) ──────────────
	jobs := make(chan ScoreJob, variables.TimeDecayJobBuffer)

	// Détermination de la limite de concurrence matérielle stricte (Ex: 8 cœurs = 8 workers)
	numWorkers := runtime.GOMAXPROCS(0)
	logger.Log.Info().Int("workers_cpu", numWorkers).Msg("Démarrage du Time-Decay Engine (Calcul des scores)")

	// ── ÉTAPE 2 : LANCEMENT DU POOL DE WORKERS (CONSOMMATEURS) ──────────────
	for i := 0; i < numWorkers; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return // Arrêt gracieux du serveur
				case job := <-jobs:
					// Conversion du booléen en multiplicateur entier
					mediaCount := 0
					if job.HasMedia {
						mediaCount = 1
					}

					// Mise à jour du score dans le Cache L1 (ZSETs algorithmiques)
					cache_service.UpdateScoreWithMetrics(
						ctx,
						job.PostID,
						job.LikeCount,
						job.CommentCount,
						job.ViewCount,
						mediaCount,
						job.CreatedAt,
						job.Hashtags,         // Tags Directs
						job.IndirectHashtags, // Tags Indirects
						job.Visibility,
						job.ReportCount,
						job.PriorityLevel,
					)
				}
			}
		}()
	}

	// ── ÉTAPE 3 : LANCEMENT DES CRONS ÉTAGÉS (PRODUCTEURS) ──────────────────
	// Pour économiser les ressources, on recalcule très souvent les posts récents,
	// et beaucoup moins souvent les posts anciens (qui ont déjà subi un fort déclin).
	go runTierCron(ctx, jobs, 2*time.Minute, "0", "6 hours")          // Tier 1 : Hyperactif
	go runTierCron(ctx, jobs, 15*time.Minute, "6 hours", "24 hours")  // Tier 2 : Actif
	go runTierCron(ctx, jobs, 60*time.Minute, "24 hours", "72 hours") // Tier 3 : Ralenti
	go runTierCron(ctx, jobs, 6*time.Hour, "72 hours", "30 days")     // Tier 4 : Dormant
}

// runTierCron exécute la récupération des métriques sur une tranche d'âge précise.
func runTierCron(ctx context.Context, jobs chan<- ScoreJob, interval time.Duration, minAge, maxAge string) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// ── FALLBACK L3 STRICT : CONTINUITÉ DE LA DONNÉE ────────────────
			// /!\ EXCEPTION ARCHITECTURALE /!\
			// On tape directement dans PostgreSQL (L3) en contournant MongoDB (L2).
			// Pourquoi ? Le Time-Decay requiert une continuité et une exhaustivité absolues
			// sur l'historique. MongoDb utilisant une éviction TTL (Warm Storage), il est
			// structurellement incapable de garantir qu'aucun post ne manque à l'appel.
			posts, err := postgres.FuncLoadPostsForTimeDecay(ctx, minAge, maxAge)
			if err != nil {
				logger.Log.Error().
					Err(err).
					Str("min_age", minAge).
					Str("max_age", maxAge).
					Msg("Échec critique du Tier Time-Decay : Impossible de lire PostgreSQL (L3)")
				continue
			}

			// ── ENVOI AUX WORKERS POUR CALCUL EN RAM ────────────────────────
			for _, p := range posts {
				jobs <- ScoreJob{
					PostID:           p.ID,
					LikeCount:        p.LikeCount,
					CommentCount:     p.CommentCount,
					ViewCount:        p.ViewCount,
					HasMedia:         p.HasMedia,
					CreatedAt:        p.CreatedAt,
					Hashtags:         p.Hashtags,
					IndirectHashtags: p.IndirectHashtags,
					Visibility:       p.Visibility,
					PriorityLevel:    p.PriorityLevel,
					ReportCount:      p.ReportCount,
				}
			}
		}
	}
}
