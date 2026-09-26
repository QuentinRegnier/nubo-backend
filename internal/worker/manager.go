package worker

import (
	"context"
	"sync"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # MANAGER : CHEF D'ORCHESTRE DE L'ASYNCHRONISME
// ############################################################################

// StartBackgroundWorkers lance tous les processus asynchrones vitaux de l'API Nubo.
// Il déploie les planificateurs (Crons) et les pools de travailleurs (Sharding) qui
// dépilent les files d'attente Redis H24.
func StartBackgroundWorkers(ctx context.Context) {
	logger.Log.Info().Msg("Démarrage du moteur de persistance et des processus asynchrones...")

	// ── ÉTAPE 1 : LANCEMENT DES MOTEURS ALGORITHMIQUES (CRONS) ──────────────

	// Moteur de Time-Decay : Recalcule la chute du score viral au fil du temps
	StartScoreUpdaterCron(ctx)

	// Moteur d'Émergence : Fusionne les hashtags similaires (Fautes de frappe)
	StartHashtagCanonCron(ctx)

	// Moteur de Tendances : Calcule le classement mondial des hashtags
	StartHashtagTrendCron(ctx)

	// Moteur de Warm-up : Régénère en silence les flux des utilisateurs inactifs
	StartFeedWarmupCron(ctx)

	// ── ÉTAPE 2 : LANCEMENT DES GARBAGE COLLECTORS (PURGES L3->L2) ──────────

	// Détruit les fichiers physiques et logs des médias orphelins
	StartMediaCleanupCron(ctx)

	// Nettoie les Likes qui pointent vers des posts supprimés
	StartLikeCleanupCron(ctx)

	// Nettoie les Réactions (Emoji) sur les messages supprimés
	StartReactionCleanupCron(ctx)

	// Supprime les sauvegardes de posts qui n'existent plus
	StartSavedCleanupCron(ctx)

	// ── ÉTAPE 3 : LANCEMENT DES CANAUX EXTERNES ─────────────────────────────

	// Initialise Firebase Cloud Messaging et consomme les requêtes de Push
	StartPushNotificationWorker(ctx)

	// ── ÉTAPE 4 : DÉPLOIEMENT DU WORKER POOL (SHARDING) ─────────────────────
	// L'infrastructure asynchrone repose sur 64 Shards Redis pour annuler
	// la contention (Locking) et traiter massivement les flux d'E/S en parallèle.

	var wg sync.WaitGroup

	for i := 0; i < redis.QueueShards; i++ {
		wg.Add(1)

		// Chaque Goroutine gère exclusivement son propre Shard
		go func(shardID int) {
			defer wg.Done()
			runWorker(ctx, shardID)
		}(i)
	}

	// Note : L'attente du WaitGroup (wg.Wait) n'est pas bloquante ici,
	// car le maintien en vie de l'application est géré directement par le serveur
	// HTTP principal (Gin) dans cmd/main.go. Les workers tourneront tant que
	// le contexte global ne recevra pas le signal d'arrêt (ctx.Done).
	logger.Log.Info().Msg("Moteur de persistance asynchrone opérationnel. Les 64 Workers sont à l'écoute.")
}
