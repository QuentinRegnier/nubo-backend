package worker

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/feed_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
)

// StartFeedWarmupCron orchestre l'auto-génération des flux d'actualités par lots pour les utilisateurs inactifs.
// S'exécute à intervalles réguliers sans jamais scanner l'intégralité de la base de données (O(log(N) + M)).
func StartFeedWarmupCron(ctx context.Context) {
	log.Println("🚀 Démarrage du Moteur de Warm-up Algorithmique (Cron 5m)...")
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processScheduledWarmups(ctx)
			}
		}
	}()
}

func processScheduledWarmups(ctx context.Context) {
	now := time.Now().Unix()
	const batchSize = 500

	// 1. Extraction chirurgicale des seuls utilisateurs arrivés à échéance (O(log(N) + M))
	expiredUserIDs, err := redis.FeedSchedule.ZRangeByScoreWithLimit(ctx, "global", now, batchSize)
	if err != nil || len(expiredUserIDs) == 0 {
		return
	}

	log.Printf("🔄 [Warm-up] Analyse d'un lot de %d utilisateurs éligibles.", len(expiredUserIDs))

	for _, idStr := range expiredUserIDs {
		userID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}

		// 2. Récupération des données de télémétrie (Mock temporaire avant raccordement Pilier 1)
		lastActiveAt, isOnline := mockGetTelemetryData(userID)

		// Si l'utilisateur est actuellement en ligne, on ne pollue pas sa session active
		if isOnline {
			// On repousse simplement sa vérification à plus tard
			_ = redis.FeedSchedule.ZAdd(ctx, "global", float64(time.Now().Add(1*time.Hour).Unix()), userID)
			continue
		}

		inactivityDuration := time.Since(lastActiveAt)

		// 3. Application de la loi de décroissance exponentielle du calcul (§4.4)
		if inactivityDuration < 2*24*time.Hour {
			// --- NIVEAU 1 : Inactivité récente (< 2 jours) ---
			// Rythme soutenu : Régénération complète 2 fois par jour (Toutes les 12 heures)
			executeBackgroundGeneration(ctx, userID)
			nextRun := time.Now().Add(12 * time.Hour).Unix()
			_ = redis.FeedSchedule.ZAdd(ctx, "global", float64(nextRun), userID)

		} else if inactivityDuration >= 2*24*time.Hour && inactivityDuration < 7*24*time.Hour {
			// --- NIVEAU 2 : Absent temporaire (Entre 2 et 7 jours) ---
			// Rythme dégradé : Régénération toutes les 48 heures pour capter les grandes tendances mondiales
			executeBackgroundGeneration(ctx, userID)
			nextRun := time.Now().Add(48 * time.Hour).Unix()
			_ = redis.FeedSchedule.ZAdd(ctx, "global", float64(nextRun), userID)

		} else {
			// --- NIVEAU 3 : Mode Dormant (>= 7 jours d'inactivity) ---
			// L'utilisateur a probablement désinstallé ou abandonné l'app.
			// Protection RAM absolue : On l'exclut de la boucle et on vide ses structures volatiles.
			log.Printf("💤 [Warm-up] Utilisateur %d classé comme DORMANT. Éviction de la RAM L1 en cours.", userID)

			// Invalidation et destruction complète de son orchestrateur d'état via la couche Domaine
			_ = algorithm_service.DeleteUserFeedState(ctx, userID)

			// Retrait définitif du planificateur pour couper les requêtes Redis obsolètes
			_ = redis.FeedSchedule.ZRem(ctx, "global", userID)
		}
	}
}

// executeBackgroundGeneration réutilise la route de service officielle pour éviter la duplication de code
func executeBackgroundGeneration(ctx context.Context, userID int64) {
	log.Printf("⚡ [Warm-up] Pré-calcul d'un flux frais pour l'utilisateur inactif %d", userID)

	// Simulation de l'input d'un Pull-to-refresh destructif forcé
	input := feed_models.GetFeedInput{
		UserID:        userID,
		Force:         true, // Déclenche la purge du filtre Cuckoo et la re-sélection des paniers
		LastSeenIndex: 0,
	}

	// Appel du cas d'usage unifié. L'underscore ignore les structures hydratées/signées (le but est purement le remplissage du cache L1)
	_, _, _, _ = feed_service.GetFeed(ctx, input)
}

// mockGetTelemetryData simule le retour de l'Edge Computing en attendant l'implémentation de la route de télémétrie
func mockGetTelemetryData(userID int64) (time.Time, bool) {
	// Valeurs codées en dur pour nos simulations d'intégration
	// TODO ajouter un vrai système de telemetry
	return time.Now().Add(-30 * time.Hour), false
}

// handleSocialFanOut intercepte les créations de posts pour distribuer l'ID
// dans les boîtes aux lettres Redis ciblées (Amis ou Abonnés).
func handleSocialFanOut(ctx context.Context, events []redis.AsyncEvent) {
	for _, evt := range events {
		// On ne cible que les créations de posts réussies
		if evt.Type == redis.EntityPost && evt.Action == redis.ActionCreate {
			postID := evt.ID

			// 1. Vérification absolue via le fallback BDD/Cache (Sécurité & Visibilité)
			// getPostWithFallback est disponible dans le package worker (défini dans most_cache_worker.go)
			p, err := getPostWithFallback(ctx, postID)
			if err != nil || p.Visibility == -1 {
				continue // Le post a été supprimé ou est introuvable entre temps
			}

			authorID := p.UserID
			var targetIDs []int64

			// 2. LE FILTRE DE VISIBILITÉ ET LE FAN-OUT HYBRIDE
			if p.Visibility == 2 {
				// ✅ CAS 1 : Post Privé (Amis Uniquement)
				// On ne l'envoie qu'aux amis (graphe bidirectionnel) pour ne pas polluer les simples abonnés.
				// (Assure-toi d'avoir implémenté GetSpeedFriends dans cache_service)
				targetIDs, err = cache_service.GetSpeedFriends(ctx, authorID)
			} else {
				// ✅ CAS 2 : Post Public ou Abonnés (Visibility 0 ou 1)
				// Protection Anti-Crash "Justin Bieber" : On compte avant de charger en RAM
				// (Assure-toi d'avoir implémenté GetFollowerCount dans cache_service, via un ZCARD par exemple)
				followerCount := cache_service.GetFollowerCount(ctx, authorID)
				if followerCount > 50000 {
					log.Printf("🛡️ [FanOut] Annulé pour le VIP %d (%d abonnés). Délégation au Most Cache Global.", authorID, followerCount)
					continue
				}

				targetIDs, err = cache_service.GetSpeedFollowers(ctx, authorID)
			}

			if err != nil {
				log.Printf("⚠️ [FanOut] Impossible de lire le graphe de l'user %d: %v", authorID, err)
				continue
			}

			if len(targetIDs) == 0 {
				continue // L'utilisateur n'a pas d'audience (Ville fantôme locale), rien à distribuer
			}

			// 3. Distribution de masse via Redis Pipeline encapsulé (DDD)
			pipe := redis.FeedsMailbox.Pipeline()
			score := float64(time.Now().UnixMilli()) // Le score chronologique absolu

			// ✅ CORRECTION : On itère sur targetIDs (qui contient les amis ou les abonnés filtrés)
			for _, followerID := range targetIDs {
				// ✅ Génération propre de la clé via le wrapper de manager.go
				mailboxKey := redis.FeedsMailbox.Key(followerID)

				// ✅ On utilise pipe.Do() pour éviter les erreurs de structure avec redisgo.Z{}
				pipe.Do(ctx, "ZADD", mailboxKey, score, postID)

				// ✅ ZREMRANGEBYRANK remplace LTRIM.
				// En supprimant du rang 0 au rang -501, on demande à Redis de ne conserver que
				// les 500 posts avec le score le plus élevé (les plus récents).
				pipe.Do(ctx, "ZREMRANGEBYRANK", mailboxKey, 0, -501)
			}

			// Exécution atomique du lot de distribution
			_, err = pipe.Exec(ctx)
			if err != nil {
				log.Printf("❌ [FanOut] Échec de l'exécution du pipeline de distribution pour le post_service %d: %v", postID, err)
			}
		}
	}
}
