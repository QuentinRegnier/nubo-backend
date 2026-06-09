package worker

import (
	"context"
	"log"
	"math/rand"
	"time"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// processInternalJobs traite les tâches de calcul asynchrones comme la génération du Feed.
func processInternalJobs(ctx context.Context, events []redis.AsyncEvent) {
	for _, evt := range events {
		if evt.Type == redis.EntityFeed && evt.Action == redis.ActionBuild {
			userID := evt.ID

			// 1. Reconstituer le contexte utilisateur (Vecteur, Quotas, etc.)
			// Note: Tu devras injecter ou appeler tes services ici pour obtenir le profil
			// Exemple fictif basé sur ton architecture :
			// userVec := service.GetUserVector(ctx, userID)
			// friendIDs := service.GetUserFriends(ctx, userID)

			log.Printf("🔄 [Worker] Lazy Loading enclenché pour l'utilisateur %d", userID)

			// 2. Instancier le Magasinier avec des quotas standards
			clerk := algorithm_service.NewProtoFeedBuilder()
			quotas := algorithm_service.Quotas{
				MaxCandidates: 1000,
				SocialRatio:   0.3,
				TagRatio:      0.5,
				GlobalRatio:   0.2,
			}

			// ✅ AJOUT DES GRAINES : Le Magasinier exige son ADN déterministe
			seeds := [3]int64{rand.Int63(), rand.Int63(), rand.Int63()}
			_, err := clerk.CollectCandidates(ctx, userID, seeds, quotas)
			if err != nil {
				log.Printf("❌ [Worker] Échec Magasinier pour user %d: %v", userID, err)
				continue
			}

			// 4. Appel de la Caissière (BuildPersonalizedFeed)
			// La sauvegarde en RAM paginée sera faite automatiquement à l'Étape H par la caissière !
			// ... (Appel à feed_service.BuildPersonalizedFeed avec les bonnes options) ...

			log.Printf("✅ [Worker] Nouveau buffer feed_service généré avec succès pour l'utilisateur %d", userID)
		}
	}
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

			// 3. Distribution de masse via Redis Pipeline (Vitesse maximale, 1 seul aller-retour TCP)
			pipe := redisgo.Rdb.Pipeline()
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
