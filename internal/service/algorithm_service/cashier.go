package algorithm_service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// PersonalizedFeedOptions regroupe les paramètres d'entrée du pipeline §4.4.
type PersonalizedFeedOptions struct {
	UserID         int64          // Pour le cache_service Redis et les logs
	UserVec        []float32      // û ∈ R^224 (normalisé L2) — nil si profil absent
	UserConfidence float64        // [0.0, 1.0] — seuil LSH: activation si > 0.70
	FriendIDs      map[int64]bool // IDs des amis directs de l'utilisateur (pour B(u,p))
	Date           time.Time      // Date de référence pour les clés ZSET
	Limit          int            // Taille du feed_service (0 = TDDFeedSize = 50)
	CandidateIDs   []int64        // ✅ Les IDs apportés par le Magasinier
	Seed           int64          // ✅ La Graine du panier pour déterminer le rythme de la Vague
	StartIndex     int            // ✅ Index de départ absolu pour la Vague de Dopamine (évite le reset de la courbe au scroll)
}

// BuildPersonalizedFeed construit le feed_service personnalisé de K posts pour un utilisateur à partir des candidats.
func BuildPersonalizedFeed(ctx context.Context, opts PersonalizedFeedOptions) ([]int64, error) {
	feedSize := opts.Limit
	if feedSize <= 0 {
		feedSize = variables.TDDFeedSize // K_feed = 50
	}

	// ── Vérification du cache_service (court-circuit si feed_service récent) ─────────────
	feedKey := fmt.Sprintf(variables.RedisKeyFeedPersonalized, opts.UserID)
	if cached, err := redisgo.Rdb.Get(ctx, feedKey).Bytes(); err == nil {
		var cachedIDs []int64
		if json.Unmarshal(cached, &cachedIDs) == nil && len(cachedIDs) > 0 {
			return cachedIDs, nil
		}
	}

	// ── ÉTAPE A: Récupération des scores de tendance du Panier ────────────
	postIDs := opts.CandidateIDs
	if len(postIDs) == 0 {
		return []int64{}, nil
	}

	dateKey := opts.Date.UTC().Format("20060102")
	zsetKey := fmt.Sprintf(variables.RedisKeyTrendGlobalDaily, dateKey)

	members := make([]string, len(postIDs))
	for i, id := range postIDs {
		members[i] = strconv.FormatInt(id, 10)
	}

	// Appel abstrait du Pipeline pour récupérer les scores existants
	scores, _ := redis.ZScores(ctx, zsetKey, members)

	trendScores := make(map[int64]float64, len(postIDs))
	for i, id := range postIDs {
		score := scores[i]
		if score == 0 {
			// Fallback : Si le post (ex: boîte aux lettres) n'est pas dans le top, on sécurise son score
			score = 1.0
		}
		trendScores[id] = score
	}

	// ── ÉTAPE B: Récupération des vecteurs de contenu ─────────────────────
	vecKeys := make([]string, len(postIDs))
	for i, id := range postIDs {
		vecKeys[i] = fmt.Sprintf(variables.RedisKeyContentVector, id)
	}

	vecVals, err := redisgo.Rdb.MGet(ctx, vecKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("[personalized] MGET content vectors: %w", err)
	}

	allCandidates := make([]PostCandidate, 0, len(postIDs))
	for i, val := range vecVals {
		if val == nil || i >= len(postIDs) {
			continue
		}
		valStr, ok := val.(string)
		if !ok {
			continue
		}
		var payload ContentVectorPayload
		if err := json.Unmarshal([]byte(valStr), &payload); err != nil {
			continue
		}
		if len(payload.V) != variables.VectorDimTotal {
			continue
		}
		allCandidates = append(allCandidates, PostCandidate{
			PostID:        postIDs[i],
			AuthorID:      payload.AuthorID,
			TrendScore:    trendScores[postIDs[i]],
			ContentVec:    payload.V,
			PriorityLevel: payload.PriorityLevel, // ✅ Lecture en O(1) de la priorité
			MatrixIdx:     len(allCandidates),
		})
	}

	if len(allCandidates) == 0 {
		return []int64{}, nil
	}

	if len(opts.UserVec) != variables.VectorDimTotal {
		return extractIDsFromCandidates(allCandidates, feedSize), nil
	}

	// ── ÉTAPE C: Pré-filtrage LSH (si confidence > 0.70) ─────────────────
	var (
		filteredCandidates []PostCandidate
		G                  []float32
		totalN             int
		useOnTheFlyMatrix  bool
	)

	if opts.UserConfidence > LSHConfidenceThreshold {
		lshHash := DefaultLSHEngine.ComputeHash(opts.UserVec)
		lshIDSet, _ := GetLSHCandidateIDs(ctx, lshHash)

		filtered := make([]PostCandidate, 0, len(lshIDSet))
		for _, c := range allCandidates {
			if lshIDSet[c.PostID] {
				filtered = append(filtered, c)
			}
		}

		if len(filtered) >= feedSize*2 {
			filteredCandidates = filtered
			for i := range filteredCandidates {
				filteredCandidates[i].MatrixIdx = i
			}
			useOnTheFlyMatrix = true
		} else {
			filteredCandidates = allCandidates
		}
	} else {
		filteredCandidates = allCandidates
	}

	// ── ÉTAPE D: Calcul des scores R(u,p) ────────────────────────────────
	for i := range filteredCandidates {
		filteredCandidates[i].PersonalScore = ComputePersonalizedScore(
			filteredCandidates[i].TrendScore,
			opts.UserVec,
			filteredCandidates[i].ContentVec,
			filteredCandidates[i].AuthorID,
			opts.FriendIDs,
			filteredCandidates[i].PriorityLevel, // ✅ Transmission
		)
	}

	// ── ÉTAPE E: Matrice de similarité G ─────────────────────────────────
	if useOnTheFlyMatrix {
		G, totalN = buildSimilarityMatrix(filteredCandidates)
	} else {
		G, totalN = getOrBuildSimMatrix(filteredCandidates)
	}

	// ── ÉTAPE F: MMR itératif — K = 50 itérations ─────────────────────
	selected := RunMMR(filteredCandidates, G, totalN, variables.TDDLambdaMMR, feedSize)

	// ── ÉTAPE G: Injection de sérendipité (La Vague de Dopamine) ──────────
	serendipPool := make([]int64, len(allCandidates))
	for i, c := range allCandidates {
		serendipPool[i] = c.PostID
	}
	// ✅ Initialisation stricte du générateur avec la Seed du panier
	rng := rand.New(rand.NewSource(opts.Seed))
	selected = InjectSerendipity(selected, serendipPool, rng, opts.StartIndex) // ✅ Transmission du décalage

	feedIDs := make([]int64, len(selected))
	for i, c := range selected {
		feedIDs[i] = c.PostID
	}

	// ── ÉTAPE H: Retour du flux calculé ──────────────────────────────────
	// La Caissière (MMR) a produit la liste finale de postIDs ordonnés.
	// Le Distributeur se chargera de l'enregistrer dans le FeedState.

	return feedIDs, nil
}

// InvalidatePersonalizedFeedCache invalide le cache_service du feed_service si le vecteur a changé significativement.
func InvalidatePersonalizedFeedCache(ctx context.Context, userID int64, oldVec, newVec []float32) bool {
	if len(oldVec) != variables.VectorDimTotal || len(newVec) != variables.VectorDimTotal {
		return false
	}

	var sumSq float64
	for i, nv := range newVec {
		diff := float64(nv - oldVec[i])
		sumSq += diff * diff
	}
	delta := math.Sqrt(sumSq)

	if delta > variables.TDDDeltaInvalid {
		feedKey := fmt.Sprintf(variables.RedisKeyFeedPersonalized, userID)
		redisgo.Rdb.Del(ctx, feedKey)
		return true
	}
	return false
}

func extractIDsFromCandidates(candidates []PostCandidate, k int) []int64 {
	if k > len(candidates) {
		k = len(candidates)
	}
	ids := make([]int64, k)
	for i := 0; i < k; i++ {
		ids[i] = candidates[i].PostID
	}
	return ids
}
