package feed_service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"time"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ─────────────────────────────────────────────────────────────────────────────────
// 1. ERREURS ET CONSTANTES
// ─────────────────────────────────────────────────────────────────────────────

var (
	// ErrCuckooSaturated est levée lorsque le Circuit Breaker détecte que l'utilisateur
	// a consommé la quasi-totalité des contenus chauds disponibles.
	ErrCuckooSaturated = errors.New("cuckoo filter saturation: high rejection rate detected")
)

const (
	// Seuil à partir duquel on considère que la recherche dans le MOST Cache est stérile.
	// Si après 200 tirages consécutifs, on n'arrive pas à injecter de nouveaux posts, on coupe.
	MaxConsecutiveRejections = 200
)

// ─────────────────────────────────────────────────────────────────────────────
// STRUCTURES MÉTIER
// ─────────────────────────────────────────────────────────────────────────────

// ScoredCandidate retient le score mathématique du post_service depuis le ZSET Redis.
// Remonté ici pour être utilisé par les méthodes de classe.
type ScoredCandidate struct {
	PostID int64
	Score  float64
}

// ProtoFeedBuilder (Le Magasinier) est l'agent chargé de faire le tour du magasin
// (Redis Lists, MOST Cache ZSETs) pour remplir le CandidateBasket.
type ProtoFeedBuilder struct {
	// Les dépendances pourront être injectées ici si besoin
}

// NewProtoFeedBuilder initialise l'agent magasinier.
func NewProtoFeedBuilder() *ProtoFeedBuilder {
	return &ProtoFeedBuilder{}
}

// CollectCandidates est la méthode centrale qui sera chargée d'exécuter les phases 2, 3 et 4
// pour retourner un panier de 1000 candidats valides.
func (pf *ProtoFeedBuilder) CollectCandidates(ctx context.Context, userID int64, quotas Quotas) (*CandidateBasket, error) {
	// Validation de sécurité initiale (Étape 1.2)
	if err := quotas.Validate(); err != nil {
		return nil, fmt.Errorf("[CollectCandidates] quotas invalides : %w", err)
	}

	basket := NewCandidateBasket(quotas.MaxCandidates)

	// On récupère les objectifs chiffrés stricts
	socialTarget, tagTarget, globalTarget := quotas.GetQuotaSizes()

	// ─────────────────────────────────────────────────────────────────────────────
	// PHASE 2 : Collecte du Socle de Confiance (Le Social)
	// ─────────────────────────────────────────────────────────────────────────────

	// ÉTAPE 2.1 : Interrogation de la LIST FIFO Redis
	// Clé du feed_service issue du Fan-Out on write
	feedKey := fmt.Sprintf("feed_service:user:%d", userID)

	// LRANGE 0 499 ramène le top 500 des posts poussés par les abonnements
	rawIDs, err := redisgo.Rdb.LRange(ctx, feedKey, 0, 499).Result()
	if err != nil {
		// FALLBACK : On ne bloque pas tout le feed_service si la liste sociale est inaccessible.
		// Une partition réseau Redis ou un timeout ne doit pas empêcher le fallback
		// sur le contenu global (Phase 3). On log l'erreur et on initialise une liste vide.
		log.Printf("⚠️ [ProtoFeedBuilder] Erreur LRange sur %s : %v", feedKey, err)
		rawIDs = []string{}
	}

	// Conversion des valeurs brutes de Redis (strings) en int64 (Snowflake IDs)
	var socialCandidateIDs []int64
	for _, strID := range rawIDs {
		if id, err := strconv.ParseInt(strID, 10, 64); err == nil {
			socialCandidateIDs = append(socialCandidateIDs, id)
		}
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// ÉTAPE 2.2 & 2.3 : Filtrage et Injection dans le Panier
	// ─────────────────────────────────────────────────────────────────────────────
	nowMillis := time.Now().UnixMilli()
	maxAgeMillis := 30 * 24 * time.Hour.Milliseconds()

	var socialAdded int // Compteur strict d'éléments réellement ajoutés

	for _, id := range socialCandidateIDs {
		// ÉTAPE 2.3 : Vérification du quota
		// Si on a atteint notre objectif de posts sociaux (ex: 300), on stoppe la collecte.
		if socialAdded >= socialTarget {
			break
		}

		// ÉTAPE 2.2 : Validation Temporelle (Fraîcheur)
		postTimestamp := (id >> variables.TimeShift) + variables.Epoch
		if nowMillis-postTimestamp > maxAgeMillis {
			continue
		}

		// ÉTAPE 2.2 : Vérification Cuckoo Filter (Déjà vu)
		if service.HasSeen(ctx, userID, id) {
			continue
		}

		// ÉTAPE 2.3 : Injection dans le panier
		// La méthode Add retourne 'true' si l'ID a bien été inséré (pas de doublon interne)
		if basket.Add(id, OriginSocial) {
			socialAdded++
		}
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// ÉTAPE 2.4 : Gestion du déficit ("Ville Fantôme")
	// ─────────────────────────────────────────────────────────────────────────────
	if socialAdded < socialTarget {
		socialDeficit := socialTarget - socialAdded
		globalTarget += socialDeficit // Transfert du déficit vers le pool Global (Découverte)

		// Optionnel mais recommandé pour tes métriques :
		log.Printf("[ProtoFeedBuilder] User %d : Déficit social de %d posts transféré au Global", userID, socialDeficit)
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// PHASE 3 : Collecte Thématique et Stochastique (Le MOST Cache)
	// ─────────────────────────────────────────────────────────────────────────────

	// ÉTAPE 3.1 : Détermination des vecteurs d'attaque (Tags affinitaires)
	// On demande au cache_service les 5 tags préférés de l'utilisateur.
	topTags, err := GetUserTopTags(ctx, userID, 5)

	if err != nil || len(topTags) == 0 {
		// CAS PAR DÉFAUT : COLD START
		// L'utilisateur n'a pas de préférences (nouvel inscrit ou vecteur réinitialisé).
		// On transfère l'intégralité du quota Thématique vers le quota Global.
		globalTarget += tagTarget
		tagTarget = 0

		// log.Printf("[ProtoFeedBuilder] Cold Start (User %d) : %d posts thématiques basculés en Global", userID, tagTarget)
	} else {
		// Le tagTarget sera réparti équitablement entre les différents tags trouvés.
		// (La logique d'échantillonnage sera codée à l'étape 3.2 et 3.3)
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// ÉTAPE 3.2 : Récupération asynchrone des candidats bruts (Goroutines)
	// ─────────────────────────────────────────────────────────────────────────────

	var wg sync.WaitGroup
	var globalCandidates []ScoredCandidate
	tagCandidates := make([][]ScoredCandidate, len(topTags)) // Un tableau par tag

	// 1. Goroutine pour le flux GLOBAL (Découverte)
	wg.Add(1)
	go func() {
		defer wg.Done()
		dateKey := time.Now().UTC().Format("20060102")
		key := fmt.Sprintf("trend:global:daily:%s", dateKey)

		// ─────────────────────────────────────────────────────────────────────
		// ÉTAPE 5.2 : Tolérance aux pannes (Timeout strict de 20ms)
		// ─────────────────────────────────────────────────────────────────────
		reqCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
		defer cancel() // Libère les ressources du timer dès la fin de la goroutine

		zResults, err := redisgo.Rdb.ZRevRangeWithScores(reqCtx, key, 0, 499).Result()
		if err != nil {
			// Si le contexte expire, err vaudra context.DeadlineExceeded.
			// On logge l'erreur sans bloquer. Le tableau restera vide et le déficit sera géré.
			log.Printf("⚠️ [ProtoFeedBuilder] Timeout ou Erreur Global ZSET ignoré silencieusement: %v", err)
			return
		}

		// Traitement en mémoire locale pour éviter les Data Races sur les headers de Slices Go
		var localGlobal []ScoredCandidate
		for _, z := range zResults {
			if id, err := strconv.ParseInt(z.Member.(string), 10, 64); err == nil {
				localGlobal = append(localGlobal, ScoredCandidate{PostID: id, Score: z.Score})
			}
		}
		globalCandidates = localGlobal
	}()

	// 2. Goroutines pour chaque TAG (Affinités)
	for i, tag := range topTags {
		wg.Add(1)
		go func(index int, tagName string) {
			defer wg.Done()
			key := fmt.Sprintf("trend:tag:%s:daily", tagName)

			// ─────────────────────────────────────────────────────────────────────
			// ÉTAPE 5.2 : Tolérance aux pannes par tag (Timeout strict de 20ms)
			// ─────────────────────────────────────────────────────────────────────
			reqCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
			defer cancel()

			zResults, err := redisgo.Rdb.ZRevRangeWithScores(reqCtx, key, 0, 499).Result()
			if err != nil {
				log.Printf("⚠️ [ProtoFeedBuilder] Timeout/Erreur Tag ZSET (%s) ignoré: %v", tagName, err)
				return
			}

			var localTag []ScoredCandidate
			for _, z := range zResults {
				if id, err := strconv.ParseInt(z.Member.(string), 10, 64); err == nil {
					localTag = append(localTag, ScoredCandidate{PostID: id, Score: z.Score})
				}
			}
			tagCandidates[index] = localTag
		}(i, tag) // On passe les variables en paramètre pour figer leur valeur dans la closure
	}

	// 3. Point de synchronisation : On attend que toutes les requêtes réseau soient finies
	wg.Wait()

	// ─────────────────────────────────────────────────────────────────────────────
	// ÉTAPE 3.3 & 3.4 : Sampler Stochastique Thermodynamique & Filtrage
	// ─────────────────────────────────────────────────────────────────────────────

	// Les températures (tau) de la fonction Softmax.
	// Plus tau est élevé, plus le choix s'aplatit vers de l'aléatoire pur (sérendipité absolue).
	// Plus tau est proche de 0, plus on sélectionne le post_service avec le score maximum (ZRANGE classique).
	tauTags := 1.5
	tauGlobal := 2.0 // Température plus haute pour le global pour favoriser la découverte

	// Variable pour suivre si le disjoncteur a sauté au cours des tirages thématiques ou globaux
	var isSaturated bool

	// -- A. ÉCHANTILLONNAGE DES TAGS AFFINITAIRES --
	var tagDeficit int
	if len(topTags) > 0 {
		baseQuotaPerTag := tagTarget / len(topTags)
		remainder := tagTarget % len(topTags)

		for i, tagPool := range tagCandidates {
			quota := baseQuotaPerTag
			if i == len(topTags)-1 {
				quota += remainder
			}
			quota += tagDeficit

			added, err := pf.sampleAndInject(ctx, userID, basket, tagPool, quota, tauTags, OriginTag)
			if errors.Is(err, ErrCuckooSaturated) {
				isSaturated = true
				break // Le disjoncteur a sauté, on arrête d'interroger le MOST Cache inutilement
			}

			if added < quota {
				tagDeficit = quota - added
			} else {
				tagDeficit = 0
			}
		}
	} else {
		tagDeficit = tagTarget
	}

	// -- B. ÉCHANTILLONNAGE DU POOL GLOBAL (Si le disjoncteur n'a pas encore sauté) --
	if !isSaturated {
		globalTarget += tagDeficit
		_, err := pf.sampleAndInject(ctx, userID, basket, globalCandidates, globalTarget, tauGlobal, OriginGlobal)
		if errors.Is(err, ErrCuckooSaturated) {
			isSaturated = true
		}
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// ÉTAPE 5.1 : FALLBACK PROFOND (La Longue Traîne MongoDB)
	// ─────────────────────────────────────────────────────────────────────────────
	// Si le disjoncteur a sauté OU que le panier n'est pas encore rempli à 100%
	// après avoir vidé le MOST Cache, on va forer la base documentaire MongoDB.
	if isSaturated || basket.Size() < quotas.MaxCandidates {
		deficitTotal := quotas.MaxCandidates - basket.Size()

		// log.Printf("🚨 [CircuitBreaker] Mode Grand Dévoreur activé pour user %d. Forage de la Longue Traîne Mongo pour %d candidats.", userID, deficitTotal)

		// Appel de la méthode de forage Mongo de la longue traîne
		err := pf.fetchLongTailMongo(ctx, userID, basket, deficitTotal)
		if err != nil {
			log.Printf("❌ [ProtoFeedBuilder] Échec critique du Fallback Profond Mongo pour user %d : %v", userID, err)
		}
	}

	return basket, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// MÉTHODES UTILITAIRES DU MAGASINIER
// ─────────────────────────────────────────────────────────────────────────────

func (pf *ProtoFeedBuilder) sampleAndInject(
	ctx context.Context,
	userID int64,
	basket *CandidateBasket,
	pool []ScoredCandidate,
	target int,
	tau float64,
	origin CandidateOrigin,
) (int, error) { // <-- On retourne maintenant une erreur potentielle
	added := 0
	consecutiveRejections := 0
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for added < target && len(pool) > 0 {
		// 1. Astuce de Stabilité Numérique (Prévenir l'Overflow)
		maxScore := pool[0].Score
		for _, c := range pool {
			if c.Score > maxScore {
				maxScore = c.Score
			}
		}

		// 2. Calcul des Poids Softmax et de la somme cumulée
		sumWeights := 0.0
		weights := make([]float64, len(pool))
		for i, c := range pool {
			w := math.Exp((c.Score - maxScore) / tau)
			weights[i] = w
			sumWeights += w
		}

		// 3. Tirage aléatoire pondéré (Roulette Wheel Selection)
		r := rng.Float64() * sumWeights
		cumSum := 0.0
		selectedIndex := len(pool) - 1

		for i, w := range weights {
			cumSum += w
			if cumSum >= r {
				selectedIndex = i
				break
			}
		}

		selectedCandidate := pool[selectedIndex]

		// 4. Retrait du candidat du pool (O(1) Swap & Pop)
		pool[selectedIndex] = pool[len(pool)-1]
		pool = pool[:len(pool)-1]

		// 5. ÉTAPE 3.4 & 5.1 : Déduplication et Circuit Breaker
		if service.HasSeen(ctx, userID, selectedCandidate.PostID) {
			consecutiveRejections++
			if consecutiveRejections >= MaxConsecutiveRejections {
				return added, ErrCuckooSaturated // Disjoncteur activé !
			}
			continue
		}

		if basket.Add(selectedCandidate.PostID, origin) {
			added++
			consecutiveRejections = 0 // Reset du compteur d'échecs dès qu'on réussit une insertion
		} else {
			consecutiveRejections++
			if consecutiveRejections >= MaxConsecutiveRejections {
				return added, ErrCuckooSaturated
			}
		}
	}

	return added, nil
}

// GetUserTopTags récupère les N tags préférés de l'utilisateur depuis un index annexe léger.
// Complexité : O(log(N) + M) avec M = limit (extrêmement rapide).
func GetUserTopTags(ctx context.Context, userID int64, limit int) ([]string, error) {
	key := fmt.Sprintf("user:top_tags:%d", userID)

	// Timeout strict de 10ms pour la récupération du profil d'attaque
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	tags, err := redisgo.Rdb.ZRevRange(reqCtx, key, 0, int64(limit-1)).Result()
	if err != nil {
		// Le Fallback de la ligne 71 dans CollectCandidates captera cette erreur
		// et basculera le quota en 100% Global Découverte.
		return nil, fmt.Errorf("timeout/erreur récupération tags: %w", err)
	}

	return tags, nil
}

// fetchLongTailMongo extrait du contenu froid et persistant depuis MongoDB pour combler le panier,
// en contournant les caches Redis de tête devenus stériles pour cet utilisateur.
func (pf *ProtoFeedBuilder) fetchLongTailMongo(ctx context.Context, userID int64, basket *CandidateBasket, countNeeded int) error {
	// Création d'un générateur de nombres aléatoires local
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	batchSize := int64(100) // Typé explicitement en int64 pour la pagination Mongo
	maxAttempts := 5        // Sécurité : évite une boucle infinie si la base est vide

	for countNeeded > 0 && maxAttempts > 0 {
		maxAttempts--

		// Astuce de Skip Aléatoire : on décale la pagination pour diversifier la sérendipité
		randomSkip := int64(rng.Intn(1000))

		// 1. Définition stricte des filtres (Seulement les posts publics)
		filter := map[string]any{
			"visibility": 0, // 0 = Public (selon tes règles métier)
		}

		// 2. Tri par date décroissante pour garder une certaine fraîcheur dans les archives
		sort := map[string]any{
			"created_at": -1,
		}

		// 3. Appel RÉEL à ton architecture de repository (calls.go)
		posts, err := mongo.MongoLoadPostsPaginated(filter, sort, randomSkip, batchSize)
		if err != nil {
			log.Printf("⚠️ [fetchLongTailMongo] Erreur lors du forage Mongo (skip: %d): %v", randomSkip, err)
			return err
		}

		// 4. Si la base ne retourne plus rien, on stoppe l'hémorragie
		if len(posts) == 0 {
			break
		}

		// 5. Traitement et injection dans le panier
		for _, p := range posts {
			if countNeeded <= 0 {
				break
			}

			// L'ID du post_service provient de ta structure domain.PostRequest
			postID := p.ID

			// Double sécurité : on valide quand même face au Cuckoo
			// (Au cas où le tirage aléatoire retombe sur un post_service très ancien mais déjà vu)
			if service.HasSeen(ctx, userID, postID) {
				continue
			}

			// Tentative d'ajout au panier. L'origine est "GLOBAL" puisqu'il s'agit de découverte
			if basket.Add(postID, OriginGlobal) {
				countNeeded--
			}
		}
	}

	return nil
}
