package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"time"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// PersonalizedFeedOptions regroupe les paramètres d'entrée du pipeline §4.4.
type PersonalizedFeedOptions struct {
	UserID         int64          // Pour le cache Redis et les logs
	UserVec        []float32      // û ∈ R^224 (normalisé L2) — nil si profil absent
	UserConfidence float64        // [0.0, 1.0] — seuil LSH: activation si > 0.70
	FriendIDs      map[int64]bool // IDs des amis directs de l'utilisateur (pour B(u,p))
	Date           time.Time      // Date de référence pour les clés ZSET
	Limit          int            // Taille du feed (0 = TDDFeedSize = 50)
}

// ─────────────────────────────────────────────────────────────────────────────
// PIPELINE PRINCIPAL — ÉTAPES A→G (TDD §4.4)
// ─────────────────────────────────────────────────────────────────────────────

// BuildPersonalizedFeed construit le feed personnalisé de K posts pour un utilisateur.
//
// TDD §4.4 — Pipeline complet:
//
//	ÉTAPE A: ZREVRANGE trend:global:daily:{date} 0 999 WITHSCORES
//	ÉTAPE B: MGET content:vec:{id_1} ... content:vec:{id_1000}
//	ÉTAPE C: LSH pré-filtrage (si userConfidence > 0.70)
//	ÉTAPE D: Calcul R(u,p) pour chaque candidat (SIMD loop)
//	ÉTAPE E: Matrice de similarité G (cache ou on-the-fly)
//	ÉTAPE F: MMR itératif K=50
//	ÉTAPE G: Injection sérendipité p=0.08
//	ÉTAPE H: Cache du résultat (SETEX feed:personalized:{user_id} 300s)
func BuildPersonalizedFeed(ctx context.Context, opts PersonalizedFeedOptions) ([]int64, error) {
	feedSize := opts.Limit
	if feedSize <= 0 {
		feedSize = variables.TDDFeedSize // K_feed = 50
	}

	// ── Vérification du cache (court-circuit si feed récent) ─────────────
	//
	// TDD §4.4: SETEX feed:personalized:{user_id} 300 {feed_ids}
	feedKey := fmt.Sprintf(variables.RedisKeyFeedPersonalized, opts.UserID)
	if cached, err := redisgo.Rdb.Get(ctx, feedKey).Bytes(); err == nil {
		var cachedIDs []int64
		if json.Unmarshal(cached, &cachedIDs) == nil && len(cachedIDs) > 0 {
			return cachedIDs, nil
		}
	}

	// ── ÉTAPE A: Récupération des candidats globaux ───────────────────────
	//
	// TDD §4.4: ZREVRANGE trend:global:daily:{YYYYMMDD} 0 999 WITHSCORES
	// → {post_id, S(p,t)} × |C| = 1000
	dateKey := opts.Date.UTC().Format("20060102")
	zsetKey := fmt.Sprintf(variables.RedisKeyTrendGlobalDaily, dateKey)

	zResults, err := redisgo.Rdb.ZRevRangeWithScores(
		ctx, zsetKey, 0, int64(variables.TDDCandidates-1),
	).Result()
	if err != nil {
		return nil, fmt.Errorf("[personalized] fetch ZSET %s: %w", zsetKey, err)
	}
	if len(zResults) == 0 {
		return []int64{}, nil
	}

	// Construction de la liste ordonnée des post IDs et de leur score S(p,t)
	postIDs := make([]int64, 0, len(zResults))
	trendScores := make(map[int64]float64, len(zResults))
	for _, z := range zResults {
		memberStr, ok := z.Member.(string)
		if !ok {
			continue
		}
		id, err := strconv.ParseInt(memberStr, 10, 64)
		if err != nil {
			continue
		}
		postIDs = append(postIDs, id)
		trendScores[id] = z.Score
	}

	// ── ÉTAPE B: Récupération des vecteurs de contenu ─────────────────────
	//
	// TDD §4.4: MGET content:vec:{post_id_1} ... content:vec:{post_id_1000}
	// → {ĉ_p} × 1000
	vecKeys := make([]string, len(postIDs))
	for i, id := range postIDs {
		vecKeys[i] = fmt.Sprintf(variables.RedisKeyContentVector, id)
	}

	vecVals, err := redisgo.Rdb.MGet(ctx, vecKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("[personalized] MGET content vectors: %w", err)
	}

	// Construire la liste des candidats avec leurs vecteurs de contenu
	allCandidates := make([]PostCandidate, 0, len(postIDs))
	for i, val := range vecVals {
		if val == nil || i >= len(postIDs) {
			continue // Cache miss: vecteur expiré ou absent
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
			continue // Vecteur malformé: ignore
		}
		allCandidates = append(allCandidates, PostCandidate{
			PostID:     postIDs[i],
			AuthorID:   payload.AuthorID,
			TrendScore: trendScores[postIDs[i]],
			ContentVec: payload.V,
			MatrixIdx:  len(allCandidates), // Index dans allCandidates (0-based)
		})
	}

	if len(allCandidates) == 0 {
		return []int64{}, nil
	}

	// ── Fallback sans profil utilisateur ─────────────────────────────────
	// Si le vecteur utilisateur est absent, retourner les posts triés par tendance.
	if len(opts.UserVec) != variables.VectorDimTotal {
		return extractIDsFromCandidates(allCandidates, feedSize), nil
	}

	// ── ÉTAPE C: Pré-filtrage LSH (si confidence > 0.70) ─────────────────
	//
	// TDD §4.5:
	//   "Calcule LSH(û): O(32·224) opérations"
	//   "Récupère lsh:bucket:{hash} pour les 32 voisins"
	//   "Calcule produits scalaires exacts sur 50–200 posts filtrés"
	//
	// Chemin standard (< 0.70): utilise tous les candidats + matrice cachée.
	// Chemin LSH (≥ 0.70): filtre les candidats + matrice on-the-fly.
	var (
		filteredCandidates []PostCandidate
		G                  []float32
		totalN             int
		useOnTheFlyMatrix  bool
	)

	if opts.UserConfidence > LSHConfidenceThreshold {
		// Chemin LSH: pré-filtrage par bucket de hachage
		//
		// TDD §4.5: LSH(û) = bin(sign(P·û)) — O(32·224) multiplications
		lshHash := DefaultLSHEngine.ComputeHash(opts.UserVec)
		lshIDSet, _ := GetLSHCandidateIDs(ctx, lshHash)

		filtered := make([]PostCandidate, 0, len(lshIDSet))
		for _, c := range allCandidates {
			if lshIDSet[c.PostID] {
				filtered = append(filtered, c)
			}
		}

		// Fallback: si trop peu de candidats depuis LSH, utiliser tous les candidats
		if len(filtered) >= feedSize*2 {
			filteredCandidates = filtered
			// Re-indexer MatrixIdx pour la matrice on-the-fly
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
	//
	// TDD §4.4: "Calcul des scores R(u,p) en Go (loop SIMD)"
	// TDD §4.2: R(u,p) = S(p,t)·[ρ·<û,ĉ_p> + (1-ρ)·A(u,p) + η·B(u,p) + η_P·r_Pearson]
	for i := range filteredCandidates {
		filteredCandidates[i].PersonalScore = ComputePersonalizedScore(
			filteredCandidates[i].TrendScore,
			opts.UserVec,
			filteredCandidates[i].ContentVec,
			filteredCandidates[i].AuthorID,
			opts.FriendIDs,
		)
	}

	// ── ÉTAPE E: Matrice de similarité G ─────────────────────────────────
	//
	// TDD §4.3:
	//   G_{ij} = <ĉ_{p_i}, ĉ_{p_j}>
	//   Chemin standard: cache mémoire 5 min (partagé entre utilisateurs)
	//   Chemin LSH: calcul on-the-fly sur ~200 candidats (rapide)
	if useOnTheFlyMatrix {
		// Chemin LSH: O(n²·N/2) avec n≈200 → O(4.5M) ≈ <1 ms
		G, totalN = buildSimilarityMatrix(filteredCandidates)
	} else {
		// Chemin standard: matrice cachée partagée 5 min — TDD §4.3
		G, totalN = getOrBuildSimMatrix(filteredCandidates)
	}

	// ── ÉTAPE F: MMR itératif — K = 50 itérations ─────────────────────
	//
	// TDD §4.3:
	//   p*_i = argmax_{p ∈ C\S_i}[λ_d·R(u,p) - (1-λ_d)·max_{p'∈S_i} G[p][p']]
	//   λ_d = 0.72, K = 50
	//   Complexité: O(K·|C|·N) ≈ O(11.2M) ≈ 2–5 ms sur AVX2 (TDD §4.3)
	selected := RunMMR(filteredCandidates, G, totalN, variables.TDDLambdaMMR, feedSize)

	// ── ÉTAPE G: Injection de sérendipité ────────────────────────────────
	//
	// TDD §4.3:
	//   p_serendip = 0.08 — "4 posts sur 50 en moyenne sont des contenus de découverte"
	//   Pool: top-N du feed global (tous les candidats disponibles)
	serendipPool := make([]int64, len(allCandidates))
	for i, c := range allCandidates {
		serendipPool[i] = c.PostID
	}
	// RNG par requête (seed temporel pour variance entre utilisateurs)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	selected = InjectSerendipity(selected, serendipPool, variables.TDDSerendipity, rng)

	// Extraction des IDs finaux (feed ordonné par MMR)
	feedIDs := make([]int64, len(selected))
	for i, c := range selected {
		feedIDs[i] = c.PostID
	}

	// ── ÉTAPE H: Mise en cache du résultat ───────────────────────────────
	//
	// TDD §4.4: SETEX feed:personalized:{user_id} 300 {feed_ids}
	// TTL = 300 s = 5 minutes
	if feedJSON, err := json.Marshal(feedIDs); err == nil {
		if err := redisgo.Rdb.SetEx(
			ctx,
			feedKey,
			feedJSON,
			time.Duration(variables.TDDFeedCacheTTL)*time.Second,
		).Err(); err != nil {
			log.Printf("⚠️ [personalized] cache feed %d: %v", opts.UserID, err)
		}
	}

	return feedIDs, nil
}

// InvalidatePersonalizedFeedCache invalide le cache du feed si le vecteur a changé significativement.
//
// TDD §4.4 — Invalidation explicite:
//
//	"Déclenchée par le handler de synchronisation de profil si le vecteur a changé
//	 significativement (variation de la norme ||û_{t+1} - û_t||_2 > δ_inval = 0.15)"
func InvalidatePersonalizedFeedCache(ctx context.Context, userID int64, oldVec, newVec []float32) bool {
	if len(oldVec) != variables.VectorDimTotal || len(newVec) != variables.VectorDimTotal {
		return false
	}

	// ||û_{t+1} - û_t||_2 — norme de la différence
	//
	// TDD §4.4: δ_inval = 0.15
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

// extractIDsFromCandidates extrait les k premiers post IDs d'une liste de candidats.
// Utilisé comme fallback lorsque le profil utilisateur est absent.
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
