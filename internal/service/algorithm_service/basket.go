package algorithm_service

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────────
// CONFIGURATION DES QUOTAS & ORIGINES
// ─────────────────────────────────────────────────────────────────────────────

// CandidateOrigin définit la provenance d'un post_service dans le panier pour l'A/B testing et les métriques.
type CandidateOrigin string

const (
	OriginSocial CandidateOrigin = "SOCIAL" // Issu des abonnements ou amis
	OriginTag    CandidateOrigin = "TAG"    // Issu des préférences thématiques de l'utilisateur
	OriginGlobal CandidateOrigin = "GLOBAL" // Issu des tendances pures (Sérendipité / Découverte)
)

// Quotas définit les règles de répartition et la taille cible du panier de candidats bruts.
type Quotas struct {
	MaxCandidates int     // Taille cible totale du panier (ex: 1000)
	SocialRatio   float64 // Proportion de posts issus du réseau d'abonnements/amis (ex: 0.3)
	TagRatio      float64 // Proportion de posts issus des affinités thématiques (ex: 0.5)
	GlobalRatio   float64 // Proportion de posts issus des tendances globales (ex: 0.2)
}

// Validate s'assure de l'exactitude mathématique et de la cohérence des quotas injectés.
func (fq *Quotas) Validate() error {
	if fq.MaxCandidates <= 0 {
		return fmt.Errorf("le nombre maximum de candidats doit être strictement positif")
	}

	// Tolérance aux imprécisions microscopiques d'arrondi des float
	sum := fq.SocialRatio + fq.TagRatio + fq.GlobalRatio
	if math.Abs(sum-1.0) > 1e-6 {
		return fmt.Errorf("la somme des ratios de distribution doit être strictement égale à 1.0 (actuellement: %f)", sum)
	}

	if fq.SocialRatio < 0 || fq.TagRatio < 0 || fq.GlobalRatio < 0 {
		return fmt.Errorf("les ratios de distribution ne peuvent pas être négatifs")
	}

	return nil
}

// GetQuotaSizes convertit les ratios en tailles absolues d'IDs à collecter.
// Sécurise le calcul pour éviter toute perte ou surplus d'unité lié aux arrondis de flottants.
func (fq *Quotas) GetQuotaSizes() (socialSize, tagSize, globalSize int) {
	socialSize = int(math.Round(float64(fq.MaxCandidates) * fq.SocialRatio))
	tagSize = int(math.Round(float64(fq.MaxCandidates) * fq.TagRatio))

	// Le dernier segment prend le reste exact pour garantir la stricte égalité avec MaxCandidates
	globalSize = fq.MaxCandidates - (socialSize + tagSize)
	return
}

// ─────────────────────────────────────────────────────────────────────────────
// LE PANIER DE CANDIDATS (BASKET)
// ─────────────────────────────────────────────────────────────────────────────

// BasketItem représente un post_service dans le panier avant son passage en "caisse" (MMR).
type BasketItem struct {
	PostID int64
	Origin CandidateOrigin
}

// BasketPersonality est l'ADN du feed, dicté exclusivement par sa Seed
type BasketPersonality struct {
	TagVariance         float64 // Modifie le quota global vs tags (Ex: ±15%)
	ExplorationExponent float64 // Si < 1 : Aventureux (lisse les poids). Si > 1 : Conservateur (accentue le top)
	RankSkew            float64 // Plus c'est élevé, plus l'aléatoire favorise les index proches de 0 (le Top)
	FreshnessBias       float64 // 0.0 à 1.0. Détermine la probabilité de piocher dans le Hourly plutôt que le Daily
}

// CandidateBasket représente UN seul panier avec son ADN
type CandidateBasket struct {
	mu        sync.RWMutex
	UniqueIDs map[int64]struct{}
	Items     []BasketItem
	Capacity  int  // ✅ Capacité maximale du panier
	isFull    bool // ✅ Flag d'optimisation

	Seed        int64
	rng         *rand.Rand
	Personality BasketPersonality // ✅ L'identité du Feed
}

// NewCandidateBasket initialise un panier et forge sa personnalité
func NewCandidateBasket(capacity int, seed int64) *CandidateBasket {
	rng := rand.New(rand.NewSource(seed))

	personality := BasketPersonality{
		// Variance de -15% à +15% sur les quotas
		TagVariance: (rng.Float64() * 0.30) - 0.15,
		// Exposant de 0.5 (très explorateur) à 2.0 (très conservateur)
		ExplorationExponent: (rng.Float64() * 1.5) + 0.5,
		// Puissance de courbure de 1.5 à 4.0. (4.0 = On tire presque toujours les premiers)
		RankSkew: (rng.Float64() * 2.5) + 1.5,
		// Biais de fraîcheur (0 = Aime les vieux posts certifiés, 1 = Aime les posts de l'heure)
		FreshnessBias: rng.Float64(),
	}

	return &CandidateBasket{
		UniqueIDs:   make(map[int64]struct{}, capacity),
		Items:       make([]BasketItem, 0, capacity),
		Capacity:    capacity, // ✅ Initialisation
		isFull:      false,    // ✅ Initialisation
		Seed:        seed,
		rng:         rng,
		Personality: personality,
	}
}

// Add tente d'insérer un post_service dans le panier (Dédoublonnage O(1) + Cuckoo Filter)
func (b *CandidateBasket) Add(ctx context.Context, userID int64, postID int64, origin CandidateOrigin) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isFull {
		return false
	}

	if _, exists := b.UniqueIDs[postID]; !exists { // ✅ Remplacé ItemsMap par UniqueIDs
		// ✅ VÉRIFICATION CUCKOO FILTER
		if service.HasSeen(ctx, userID, postID) {
			return false
		}

		b.Items = append(b.Items, BasketItem{PostID: postID, Origin: origin}) // ✅ Remplacé CandidateItem par BasketItem
		b.UniqueIDs[postID] = struct{}{}                                      // ✅ Remplacé ItemsMap par UniqueIDs

		if len(b.Items) >= b.Capacity {
			b.isFull = true
		}
		return true
	}
	return false
}

// Size retourne la taille (Inchangé)
func (b *CandidateBasket) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.Items)
}

// ✅ LE CŒUR DU DÉTERMINISME ULTRA-OPTIMISÉ
// FetchDeterministicallyFromZSET utilise la personnalité du panier (Seed + Skew) pour extraire
// des posts de manière pseudo-aléatoire mais 100% déterministe, favorisant le haut du classement.
func (b *CandidateBasket) FetchDeterministicallyFromZSET(ctx context.Context, userID int64, key string, targetCount int, origin CandidateOrigin) int {
	if targetCount <= 0 { // ✅ Remplacé count par targetCount
		return 0
	}

	// 1. On regarde la taille du rayon dans le magasin
	total, err := redis.ZCard(ctx, key)
	if err != nil || total == 0 {
		return 0
	}

	// 2. On génère nos index (Ranks) avec notre ADN
	// On en demande un peu plus (+50%) au pipeline pour pallier aux doublons (déjà dans le panier)
	fetchCount := int(float64(targetCount) * 1.5) // ✅ Remplacé count par targetCount
	if fetchCount > int(total) {
		fetchCount = int(total)
	}

	added := 0
	targetRanksMap := make(map[int64]struct{}) // Garde en mémoire les index déjà tentés
	maxAttempts := int(total) * 3              // Sécurité anti-boucle infinie

	// ✅ BOUCLE DE COMPENSATION : On continue tant qu'on n'a pas le quota ET qu'il reste des index inédits à tirer
	for added < targetCount && len(targetRanksMap) < int(total) && maxAttempts > 0 { // ✅ Remplacé fetchCount par targetCount
		batchSize := targetCount - added // ✅ Remplacé fetchCount par targetCount
		ranksToFetch := make([]int64, 0, batchSize)

		for len(ranksToFetch) < batchSize && len(targetRanksMap) < int(total) {
			maxAttempts--
			// MAGIE MATHÉMATIQUE (Déterministe via Seed)
			normalizedRand := math.Pow(b.rng.Float64(), b.Personality.RankSkew)
			rank := int64(normalizedRand * float64(total))
			if rank >= total {
				rank = total - 1
			}

			if _, exists := targetRanksMap[rank]; !exists {
				targetRanksMap[rank] = struct{}{}
				ranksToFetch = append(ranksToFetch, rank)
			}
		}

		if len(ranksToFetch) == 0 {
			break // ZSET totalement épuisé
		}

		// 3. Extraction Chirurgicale via Pipeline abstrait
		results, _ := redis.ZRevRangeByRanks(ctx, key, ranksToFetch)

		// 4. Dépouillement
		for _, res := range results {
			if id, err := strconv.ParseInt(res, 10, 64); err == nil {
				// b.Add gère l'anti-doublon en interne et le Cuckoo Filter.
				if b.Add(ctx, userID, id, origin) {
					added++
				}
			}
		}
	}

	return added // Retourne le nombre exact de posts réellement ajoutés
}

// ─────────────────────────────────────────────────────────────────────────────
// LE CHARIOT (Gestionnaire des 3 Paniers)
// ─────────────────────────────────────────────────────────────────────────────

// FeedBaskets orchestre la création simultanée des Feeds A, B et C
type FeedBaskets struct {
	A *CandidateBasket
	B *CandidateBasket
	C *CandidateBasket
}

// NewFeedBaskets crée le chariot avec les 3 graines générées à l'Étape 2
func NewFeedBaskets(capacity int, seedA, seedB, seedC int64) *FeedBaskets {
	return &FeedBaskets{
		A: NewCandidateBasket(capacity, seedA),
		B: NewCandidateBasket(capacity, seedB),
		C: NewCandidateBasket(capacity, seedC),
	}
}

// ✅ ACTION 1 : Aspiration de la boîte aux lettres sociale
// LoadSocialMailbox lit le ZSET préparé par le Worker et injecte TOUS les posts
// des abonnements dans les 3 paniers sans distinction (Valeurs sûres).
func (fb *FeedBaskets) LoadSocialMailbox(ctx context.Context, userID int64) error {
	mailboxKey := redis.FeedsMailbox.Key(userID)

	idStrings, err := redis.ZRevRange(ctx, mailboxKey, 0, -1)
	if err != nil {
		return err
	}

	for _, idStr := range idStrings {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			fb.A.Add(ctx, userID, id, OriginSocial)
			fb.B.Add(ctx, userID, id, OriginSocial)
			fb.C.Add(ctx, userID, id, OriginSocial)
		}
	}

	// ✅ VIDAGE DE LA BOÎTE AUX LETTRES
	// On la purge pour éviter que la prochaine re-sélection (Cas 3) reprenne les mêmes posts.
	_ = redis.Del(ctx, mailboxKey)

	return nil
}
