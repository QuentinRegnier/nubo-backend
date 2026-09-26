package algorithm_service

import (
	"context"
	"math"
	"math/rand"
	"strconv"
	"sync"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
)

// ############################################################################
// # SECTION 1 : CONFIGURATION DES QUOTAS ET ORIGINES
// ############################################################################

// CandidateOrigin définit la provenance d'un post dans le panier pour l'A/B testing et les métriques.
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
func (quotas *Quotas) Validate() error {
	if quotas.MaxCandidates <= 0 {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Le nombre maximum de candidats doit être strictement positif.", nil)
	}

	// Tolérance aux imprécisions microscopiques d'arrondi des float
	sumOfRatios := quotas.SocialRatio + quotas.TagRatio + quotas.GlobalRatio
	if math.Abs(sumOfRatios-1.0) > 1e-6 {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "La somme des ratios de distribution doit être strictement égale à 1.0.", nil)
	}

	if quotas.SocialRatio < 0 || quotas.TagRatio < 0 || quotas.GlobalRatio < 0 {
		return nubo_error.NewBadRequest(nubo_error.CodeInvalidPayload, "Les ratios de distribution ne peuvent pas être négatifs.", nil)
	}

	return nil
}

// GetQuotaSizes convertit les ratios en tailles absolues d'IDs à collecter.
// Sécurise le calcul pour éviter toute perte ou surplus d'unité lié aux arrondis de flottants.
func (quotas *Quotas) GetQuotaSizes() (socialSize, tagSize, globalSize int) {
	socialSize = int(math.Round(float64(quotas.MaxCandidates) * quotas.SocialRatio))
	tagSize = int(math.Round(float64(quotas.MaxCandidates) * quotas.TagRatio))

	// Le dernier segment prend le reste exact pour garantir la stricte égalité avec MaxCandidates
	globalSize = quotas.MaxCandidates - (socialSize + tagSize)
	return socialSize, tagSize, globalSize
}

// ############################################################################
// # SECTION 2 : LE PANIER DE CANDIDATS (BASKET)
// ############################################################################

// BasketItem représente un post dans le panier avant son passage en "caisse" (MMR).
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
	mutex            sync.RWMutex
	UniquePostIDsMap map[int64]struct{} // Set pour dédoublonnage en O(1)
	Items            []BasketItem
	Capacity         int
	IsFull           bool

	Seed        int64
	RandomGen   *rand.Rand
	Personality BasketPersonality
}

// NewCandidateBasket initialise un panier et forge sa personnalité de manière déterministe
func NewCandidateBasket(capacity int, seed int64) *CandidateBasket {
	randomGen := rand.New(rand.NewSource(seed))

	personality := BasketPersonality{
		// Variance de -15% à +15% sur les quotas
		TagVariance: (randomGen.Float64() * 0.30) - 0.15,
		// Exposant de 0.5 (très explorateur) à 2.0 (très conservateur)
		ExplorationExponent: (randomGen.Float64() * 1.5) + 0.5,
		// Puissance de courbure de 1.5 à 4.0. (4.0 = On tire presque toujours les premiers)
		RankSkew: (randomGen.Float64() * 2.5) + 1.5,
		// Biais de fraîcheur (0 = Aime les vieux posts certifiés, 1 = Aime les posts de l'heure)
		FreshnessBias: randomGen.Float64(),
	}

	return &CandidateBasket{
		UniquePostIDsMap: make(map[int64]struct{}, capacity),
		Items:            make([]BasketItem, 0, capacity),
		Capacity:         capacity,
		IsFull:           false,
		Seed:             seed,
		RandomGen:        randomGen,
		Personality:      personality,
	}
}

// Add tente d'insérer un post dans le panier (Dédoublonnage O(1) + Cuckoo Filter)
func (basket *CandidateBasket) Add(ctx context.Context, userID int64, postID int64, origin CandidateOrigin) bool {
	basket.mutex.Lock()
	defer basket.mutex.Unlock()

	if basket.IsFull {
		return false
	}

	if _, exists := basket.UniquePostIDsMap[postID]; !exists {
		// VÉRIFICATION CUCKOO FILTER (L'utilisateur l'a-t-il déjà vu ?)
		if service.HasSeen(ctx, userID, postID) {
			return false
		}

		basket.Items = append(basket.Items, BasketItem{PostID: postID, Origin: origin})
		basket.UniquePostIDsMap[postID] = struct{}{}

		if len(basket.Items) >= basket.Capacity {
			basket.IsFull = true
		}
		return true
	}
	return false
}

// Size retourne la taille actuelle du panier de manière thread-safe
func (basket *CandidateBasket) Size() int {
	basket.mutex.RLock()
	defer basket.mutex.RUnlock()
	return len(basket.Items)
}

// FetchDeterministicallyFromZSET utilise la personnalité du panier (Seed + Skew) pour extraire
// des posts de manière pseudo-aléatoire mais 100% déterministe, favorisant le haut du classement.
func (basket *CandidateBasket) FetchDeterministicallyFromZSET(ctx context.Context, userID int64, zsetKey string, targetCount int, origin CandidateOrigin) int {
	if targetCount <= 0 {
		return 0
	}

	// 1. Déterminer la taille totale disponible dans le rayon (ZSET)
	totalElements, err := redis.ZCard(ctx, zsetKey)
	if err != nil || totalElements == 0 {
		return 0
	}

	// 2. Génération des Ranks avec l'ADN du panier
	// On demande +50% au pipeline pour compenser les doublons potentiels (déjà en panier ou vus)
	fetchBatchSize := int(float64(targetCount) * 1.5)
	if fetchBatchSize > int(totalElements) {
		fetchBatchSize = int(totalElements)
	}

	successfullyAddedCount := 0
	attemptedRanksMap := make(map[int64]struct{})
	maxSafetyAttempts := int(totalElements) * 3

	// Boucle de compensation : on pioche tant qu'il nous manque des posts et qu'on n'a pas épuisé le ZSET
	for successfullyAddedCount < targetCount && len(attemptedRanksMap) < int(totalElements) && maxSafetyAttempts > 0 {
		currentBatchSize := targetCount - successfullyAddedCount
		ranksToFetch := make([]int64, 0, currentBatchSize)

		for len(ranksToFetch) < currentBatchSize && len(attemptedRanksMap) < int(totalElements) {
			maxSafetyAttempts--

			// MAGIE MATHÉMATIQUE : Courbure de la probabilité via le RankSkew
			normalizedRandomValue := math.Pow(basket.RandomGen.Float64(), basket.Personality.RankSkew)
			rank := int64(normalizedRandomValue * float64(totalElements))
			if rank >= totalElements {
				rank = totalElements - 1
			}

			if _, alreadyAttempted := attemptedRanksMap[rank]; !alreadyAttempted {
				attemptedRanksMap[rank] = struct{}{}
				ranksToFetch = append(ranksToFetch, rank)
			}
		}

		if len(ranksToFetch) == 0 {
			break // Le ZSET est totalement épuisé
		}

		// 3. Extraction Chirurgicale via Pipeline abstrait
		results, _ := redis.ZRevRangeByRanks(ctx, zsetKey, ranksToFetch)

		// 4. Dépouillement et ajout au panier
		for _, rawID := range results {
			if parsedID, err := strconv.ParseInt(rawID, 10, 64); err == nil {
				// basket.Add gère l'anti-doublon en interne et le Cuckoo Filter
				if basket.Add(ctx, userID, parsedID, origin) {
					successfullyAddedCount++
				}
			}
		}
	}

	return successfullyAddedCount
}

// ############################################################################
// # SECTION 3 : LE CHARIOT (Gestionnaire des Feeds Parallèles)
// ############################################################################

// FeedBaskets orchestre la création simultanée des Feeds A, B et C
type FeedBaskets struct {
	FeedA *CandidateBasket
	FeedB *CandidateBasket
	FeedC *CandidateBasket
}

// NewFeedBaskets crée le chariot avec les 3 graines générées pour la session
func NewFeedBaskets(capacity int, seedA, seedB, seedC int64) *FeedBaskets {
	return &FeedBaskets{
		FeedA: NewCandidateBasket(capacity, seedA),
		FeedB: NewCandidateBasket(capacity, seedB),
		FeedC: NewCandidateBasket(capacity, seedC),
	}
}

// LoadSocialMailbox lit le ZSET préparé par le Worker asynchrone et injecte TOUS les posts
// des abonnements/amis dans les 3 paniers sans distinction (Valeurs sûres).
func (feedBaskets *FeedBaskets) LoadSocialMailbox(ctx context.Context, userID int64) error {
	mailboxKey := redis.FeedsMailbox.Key(userID)

	idStrings, err := redis.ZRevRange(ctx, mailboxKey, 0, -1)
	if err != nil {
		// On masque l'erreur Redis brute sous une AppError standardisée
		return nubo_error.NewInternal()
	}

	for _, idStr := range idStrings {
		if postID, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			feedBaskets.FeedA.Add(ctx, userID, postID, OriginSocial)
			feedBaskets.FeedB.Add(ctx, userID, postID, OriginSocial)
			feedBaskets.FeedC.Add(ctx, userID, postID, OriginSocial)
		}
	}

	// VIDAGE DE LA BOÎTE AUX LETTRES
	// On la purge pour éviter que la prochaine génération ne recycle les mêmes posts.
	_ = redis.Del(ctx, mailboxKey)

	return nil
}
