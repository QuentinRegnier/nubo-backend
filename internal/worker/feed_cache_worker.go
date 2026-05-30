package worker

import (
	"context"
	"fmt"
	"log"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed"
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
			clerk := feed.NewProtoFeedBuilder()
			quotas := feed.Quotas{
				MaxCandidates: 1000,
				SocialRatio:   0.3,
				TagRatio:      0.5,
				GlobalRatio:   0.2,
			}

			// 3. Exécuter le pipeline complet silencieusement
			_, err := clerk.CollectCandidates(ctx, userID, quotas)
			if err != nil {
				log.Printf("❌ [Worker] Échec Magasinier pour user %d: %v", userID, err)
				continue
			}

			// 4. Appel de la Caissière (BuildPersonalizedFeed)
			// La sauvegarde en RAM paginée sera faite automatiquement à l'Étape H par la caissière !
			// ... (Appel à feed.BuildPersonalizedFeed avec les bonnes options) ...

			log.Printf("✅ [Worker] Nouveau buffer feed généré avec succès pour l'utilisateur %d", userID)
		}
	}
}

// handleSocialFanOut intercepte les créations de posts pour distribuer l'ID
// dans les boîtes aux lettres Redis de tous les abonnés de l'auteur.
func handleSocialFanOut(ctx context.Context, events []redis.AsyncEvent) {
	for _, evt := range events {
		// On ne cible que les créations de posts réussies
		if evt.Type == redis.EntityPost && evt.Action == redis.ActionCreate {
			postID := evt.ID

			// Extraction sécurisée de l'ID de l'auteur depuis le payload de l'événement
			var authorID int64
			if payloadMap, ok := evt.Payload.(map[string]any); ok {
				if uid, found := payloadMap["user_id"]; found {
					switch v := uid.(type) {
					case float64:
						authorID = int64(v)
					case int64:
						authorID = v
					}
				}
			}

			if authorID == 0 {
				continue // Impossible de déterminer l'auteur, protection contre les payloads corrompus
			}

			// 1. Récupération instantanée des abonnés depuis le Speed Cache (O(1) en RAM)
			followersKey := fmt.Sprintf("speed:followers:%d", authorID)
			followerStrings, err := redisgo.Rdb.SMembers(ctx, followersKey).Result()
			if err != nil {
				log.Printf("⚠️ [FanOut] Impossible de lire les abonnés de l'user %d: %v", authorID, err)
				continue
			}

			if len(followerStrings) == 0 {
				continue // L'utilisateur n'a pas d'abonnés (Ville fantôme locale), rien à distribuer
			}

			// 2. Distribution de masse via Redis Pipeline (Vitesse maximale, 1 seul aller-retour TCP)
			pipe := redisgo.Rdb.Pipeline()
			for _, followerStr := range followerStrings {
				// Génération de la clé de la boîte aux lettres du flux social de l'abonné
				feedUserKey := fmt.Sprintf("feed:user:%s", followerStr)

				// LPUSH met le nouveau post tout en haut de la file d'attente sociale
				pipe.LPush(ctx, feedUserKey, postID)

				// LTRIM borne la boîte aux lettres à 1000 éléments.
				// Cela évite que le panier social d'un utilisateur inactif ne grandisse indéfiniment en RAM.
				pipe.LTrim(ctx, feedUserKey, 0, 999)
			}

			// Exécution atomique du lot de distribution
			_, err = pipe.Exec(ctx)
			if err != nil {
				log.Printf("❌ [FanOut] Échec de l'exécution du pipeline de distribution pour le post %d: %v", postID, err)
			}
		}
	}
}
