package feed_service

import (
	"context"
	"fmt"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
)

// FeedDistributor orchestre la distribution et le cycle de vie du flux d'actualité.
type FeedDistributor struct {
	clerk *ProtoFeedBuilder
}

// NewFeedDistributor initialise le distributeur de flux.
func NewFeedDistributor(clerk *ProtoFeedBuilder) *FeedDistributor {
	return &FeedDistributor{
		clerk: clerk,
	}
}

// RefreshOptions encapsule les données de contexte envoyées par le client mobile/web.
type RefreshOptions struct {
	UserID        int64
	LastSeenIndex int     // L'index maximal atteint par l'utilisateur dans son défilement actuel
	SeenPostIDs   []int64 // Les IDs exacts des posts affichés à l'écran avant le refresh
	PersonalOpts  PersonalizedFeedOptions
	Quotas        Quotas
}

// HandlePullToRefresh applique les règles psychologiques et techniques du Swipe Down.
func (d *FeedDistributor) HandlePullToRefresh(ctx context.Context, opts RefreshOptions) ([]int64, error) {
	// Seuil critique métier issu des spécifications (§4.2)
	const DislikeThreshold = 10

	// ─────────────────────────────────────────────────────────────────────────────
	// CAS 1 : REJET DU FEED (last_seen_index < 10)
	// ─────────────────────────────────────────────────────────────────────────────
	if opts.LastSeenIndex < DislikeThreshold {
		// 1. Inscription immédiate dans le Cuckoo Filter pour l'exclusion définitive
		for _, postID := range opts.SeenPostIDs {
			service.MarkAsSeen(ctx, opts.UserID, postID)
		}

		// 2. Destruction de l'ancien buffer décevant
		if err := ClearBuffer(ctx, opts.UserID); err != nil {
			log.Printf("⚠️ [Distributor] Échec de la purge du buffer pour l'user %d : %v", opts.UserID, err)
		}

		// 3. Appel synchrone au Magasinier pour collecter 1000 nouveaux candidats viables
		// Le fait d'avoir marqué les posts précédents comme vus garantit mathématiquement
		// qu'ils ne se retrouveront pas dans ce nouveau panier.
		_, err := d.clerk.CollectCandidates(ctx, opts.UserID, opts.Quotas)
		if err != nil {
			return nil, err
		}

		// 4. Passage en Caisse (Calcul vectoriel, MMR, Sauvegarde du buffer et retour Page 1)
		// On délègue à BuildPersonalizedFeed le traitement lourd de scoring.
		freshFeedPage, err := BuildPersonalizedFeed(ctx, opts.PersonalOpts)
		if err != nil {
			return nil, err
		}

		return freshFeedPage, nil
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// CAS 2 : CONSOMMATION NORMALE (last_seen_index >= 10)
	// ─────────────────────────────────────────────────────────────────────────────

	currentPage := GetCurrentCursor(ctx, opts.UserID)
	nextPage := currentPage + 1

	// 2. Tentative de chargement de la page suivante précalculée
	nextPageIDs, err := GetBufferPage(ctx, opts.UserID, nextPage)
	if err == nil {
		IncrementCursor(ctx, opts.UserID)

		// ─────────────────────────────────────────────────────────────────────
		// ÉTAPE 4.3 : LE LAZY LOADING (Anticipation Asynchrone)
		// ─────────────────────────────────────────────────────────────────────
		// On vérifie si la page N+1 existe toujours. Si elle n'existe pas,
		// l'utilisateur est sur sa dernière cartouche.
		futurePageKey := fmt.Sprintf(RedisKeyFeedBufferPage, opts.UserID, nextPage+1)

		// Note : Exists() est O(1), ça ne coûte rien. On évalue directement le booléen.
		exists, _ := redis.Exists(ctx, futurePageKey)
		if !exists {

			// On poste l'événement dans ta file Redis shardée.
			// partitionKey = opts.UserID pour que ça aille dans le bon shard.
			// Target = TargetWorker pour ne pas polluer Mongo/Postgres.
			// On utilise context.Background() pour que l'envoi ne soit pas annulé si la requête HTTP se termine.
			errEnqueue := redis.EnqueueDB(
				context.Background(),
				opts.UserID,
				opts.UserID,
				redis.EntityFeed,
				redis.ActionBuild,
				nil, // Pas besoin de payload, le worker refera un fetch complet de l'utilisateur
				redis.TargetWorker,
			)

			if errEnqueue != nil {
				log.Printf("⚠️ [Distributor] Échec déclenchement Lazy Loading user %d: %v", opts.UserID, errEnqueue)
			}
		}

		return nextPageIDs, nil
	}

	// 3. Fallback de secours (Buffer épuisé ou expiré)
	// ... (le code existant de fallback reste inchangé)
	// Si la page suivante n'existe pas en cache_service, on reconstruit proprement un flux complet.
	log.Printf("[Distributor] Buffer épuisé pour l'user %d à la page %d. Déclenchement d'une régénération automatique.", opts.UserID, nextPage)

	_, err = d.clerk.CollectCandidates(ctx, opts.UserID, opts.Quotas)
	if err != nil {
		return nil, err
	}

	return BuildPersonalizedFeed(ctx, opts.PersonalOpts)
}
