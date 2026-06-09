package algorithm_service

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// PILIER 3 — ALGORITHME DE RECOMMANDATION PERSONNALISÉE
// TDD §4.2, §4.3, §4.4, §4.5
// ============================================================================

// PostCandidate représente un post_service candidat dans le pipeline de feed_service personnalisé.
type PostCandidate struct {
	PostID        int64
	AuthorID      int64
	TrendScore    float64   // S(p,t) — issu du ZSET Redis
	ContentVec    []float32 // ĉ_p ∈ R^224 (normalisé L2)
	PriorityLevel int       // ✅ Niveau de priorité (0=Normal, 1=Certifié, etc.)
	PersonalScore float64   // R(u,p) — calculé à l'étape D
	MatrixIdx     int       // Index dans la matrice de similarité G
	IsSerendipity bool      // true si injecté par le mécanisme de sérendipité
}

// ─────────────────────────────────────────────────────────────────────────────
// CACHE DE LA MATRICE DE SIMILARITÉ — TDD §4.3
// "calculée une fois par fenêtre temporelle (toutes les 5 minutes) et mise en
// cache_service en mémoire dans le process Go"
// ─────────────────────────────────────────────────────────────────────────────

type simMatrixState struct {
	G         []float32 // Matrice G[n×n] aplatie row-major: G[i*n+j] = <ĉ_{p_i}, ĉ_{p_j}>
	n         int
	postIDs   []int64 // Liste ordonnée des post_service IDs utilisés pour construire G
	expiresAt time.Time
}

var (
	simCacheMu sync.RWMutex
	simCache   *simMatrixState
)

// ─────────────────────────────────────────────────────────────────────────────
// FONCTIONS DE CALCUL VECTORIEL — SIMD-FRIENDLY
// ─────────────────────────────────────────────────────────────────────────────

// dotProductN calcule le produit scalaire de deux vecteurs de même longueur.
//
// TDD §4.2:
//
//	<û, ĉ_p> = Σ_{k=1}^{N} u_k · c_{p,k}
//
// Implémentation SIMD-friendly: le compilateur Go émet des instructions AVX2
// sur x86-64 pour cette forme de boucle (TDD §4.2 recommandation explicite).
func dotProductN(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0.0 // Garantit l'accélération matérielle (BCE)
	}
	var dot float32
	for i, v := range a {
		dot += v * b[i]
	}
	return dot
}

// ComputeSocialAffinity calcule le score d'affinité sociale A(u,p).
//
// TDD §4.2:
//
//	A(u,p) = (<û^(soc), ĉ_p^(soc)> + 1) / 2
//
// La translation (+1)/2 normalise le produit scalaire de [-1, 1] vers [0, 1].
// Les sous-blocs sociaux sont des sous-slices des vecteurs normalisés complets.
func ComputeSocialAffinity(userVec, contentVec []float32) float64 {
	if len(userVec) < variables.VectorDimTotal || len(contentVec) < variables.VectorDimTotal {
		return 0.5 // Valeur neutre si vecteurs incomplets
	}

	// Extraction des sous-blocs sociaux û^(soc) ∈ R^64 et ĉ_p^(soc) ∈ R^64
	uSoc := userVec[variables.VectorOffSoc : variables.VectorOffSoc+variables.VectorDimSoc]
	cSoc := contentVec[variables.VectorOffSoc : variables.VectorOffSoc+variables.VectorDimSoc]

	// <û^(soc), ĉ_p^(soc)> — produit scalaire sur le sous-espace social
	dot := dotProductN(uSoc, cSoc)

	// A(u,p) = (<û^(soc), ĉ_p^(soc)> + 1) / 2 ∈ [0, 1]
	return (float64(dot) + 1.0) / 2.0
}

// ComputePearsonEngagement calcule la corrélation de Pearson sur le bloc d'engagement.
//
// TDD §4.2:
//
//	r_Pearson(u,p) = Σ_k(u^(eng)_k - ū^(eng))(c^(eng)_{p,k} - c̄^(eng)_p) /
//	                 sqrt(Σ_k(u^(eng)_k - ū^(eng))² · Σ_k(c^(eng)_{p,k} - c̄^(eng)_p)²)
//
// Mesure la cohérence comportementale entre profil utilisateur et contenu.
// Retourne 0.0 si l'un des vecteurs est constant (variance nulle).
func ComputePearsonEngagement(userEng, contentEng []float32) float64 {
	const n = variables.VectorDimEng // = 8

	// Calcul des moyennes ū^(eng) et c̄^(eng)
	var meanU, meanC float64
	for i := 0; i < n; i++ {
		meanU += float64(userEng[i])
		meanC += float64(contentEng[i])
	}
	meanU /= float64(n)
	meanC /= float64(n)

	// Calcul du numérateur et des sommes de carrés au dénominateur
	var num, sumSqU, sumSqC float64
	for i := 0; i < n; i++ {
		du := float64(userEng[i]) - meanU
		dc := float64(contentEng[i]) - meanC
		num += du * dc
		sumSqU += du * du
		sumSqC += dc * dc
	}

	// Dénominateur: sqrt(Σ(u-ū)² · Σ(c-c̄)²)
	denom := math.Sqrt(sumSqU * sumSqC)
	if denom < 1e-10 {
		return 0.0 // Variance nulle (vecteur constant): corrélation indéfinie → neutre
	}

	return num / denom
}

// ComputePersonalizedScore calcule R(u, p) — le score de pertinence personnalisée.
//
// TDD §4.2:
//
//	R(u,p) = S(p,t) · [ρ · <û, ĉ_p> + (1-ρ) · A(u,p) + η · B(u,p) + η_P · r_Pearson(u,p)]
//
// Paramètres:
//   - trendScore: S(p,t) issu du ZSET Redis
//   - userVec: û ∈ R^224 (normalisé L2)
//   - contentVec: ĉ_p ∈ R^224 (normalisé L2)
//   - authorID: pour le calcul de B(u,p) (ami direct)
//   - friendIDs: set des amis directs (nil = pas d'amis directs connus)
func ComputePersonalizedScore(
	trendScore float64,
	userVec, contentVec []float32,
	authorID int64,
	friendIDs map[int64]bool,
	priorityLevel int, // ✅ Ajout du paramètre
) float64 {
	if len(userVec) != variables.VectorDimTotal || len(contentVec) != variables.VectorDimTotal {
		return trendScore * (1.0 + float64(priorityLevel)*0.5) // Fallback avec priorité
	}

	// ── <û, ĉ_p> — Similarité cosinus (produit scalaire de vecteurs normalisés)
	//
	// TDD §4.2:
	//   <û, ĉ_p> = Σ_{k=1}^{224} u_k · c_{p,k}  ∈ [-1, 1]
	//   (équivalent à cos(u, c_p) car vecteurs normalisés — TDD §2.2)
	cosineSim := float64(dotProductN(userVec, contentVec))

	// ── A(u,p) — Score d'affinité sociale
	//
	// TDD §4.2:
	//   A(u,p) = (<û^(soc), ĉ_p^(soc)> + 1) / 2  ∈ [0, 1]
	socialAffinity := ComputeSocialAffinity(userVec, contentVec)

	// ── B(u,p) — Indicateur de post_service d'un ami direct
	//
	// TDD §4.2: B(u,p) ∈ {0, 1}
	var friendBoost float64
	if friendIDs != nil && friendIDs[authorID] {
		friendBoost = 1.0
	}

	// ── r_Pearson(u,p) — Corrélation de Pearson sur le bloc engagement
	//
	// TDD §4.2: Mesure la cohérence comportementale
	uEng := userVec[variables.VectorOffEng : variables.VectorOffEng+variables.VectorDimEng]
	cEng := contentVec[variables.VectorOffEng : variables.VectorOffEng+variables.VectorDimEng]
	pearson := ComputePearsonEngagement(uEng, cEng)

	// ── Formule composite R(u,p)
	//
	// TDD §4.2:
	//   R(u,p) = S(p,t) · [ρ · <û,ĉ_p> + (1-ρ) · A(u,p) + η · B(u,p) + η_P · r_Pearson]
	//   ρ = 0.65,  η = 0.20,  η_P = 0.10
	inner := variables.TDDRho*cosineSim +
		(1.0-variables.TDDRho)*socialAffinity +
		variables.TDDEta*friendBoost +
		variables.TDDEtaP*pearson

	baseScore := trendScore * inner

	// ✅ APPLICATION DU MULTIPLICATEUR DE PRIORITÉ
	// Score final = Score * (1 + (PriorityLevel * 0.5))
	priorityMultiplier := 1.0 + (float64(priorityLevel) * 0.5)

	return baseScore * priorityMultiplier
}

// ─────────────────────────────────────────────────────────────────────────────
// MATRICE DE SIMILARITÉ — TDD §4.3
// ─────────────────────────────────────────────────────────────────────────────

// buildSimilarityMatrix construit la matrice G ∈ R^{n×n} aplatie en row-major.
//
// TDD §4.3:
//
//	G_{ij} = <ĉ_{p_i}, ĉ_{p_j}>,  i,j ∈ {1,...,n}
//
// Optimisation: seul le triangle supérieur est calculé (G est symétrique,
// G_{ii} = 1 pour vecteurs normalisés).
//
// Complexité: O(n²·N/2) = O(1000²·224/2) = O(112M) ≈ 11 ms sur AVX2.
func buildSimilarityMatrix(candidates []PostCandidate) ([]float32, int) {
	n := len(candidates)
	if n == 0 {
		return nil, 0
	}

	// Allocation de la matrice aplatie G[n×n] (row-major)
	// TDD §4.3: G ∈ R^{|C|×|C|},  |C| = 1000
	G := make([]float32, n*n)

	for i := 0; i < n; i++ {
		// G[i][i] = <ĉ_{p_i}, ĉ_{p_i}> = 1 (vecteurs normalisés L2)
		G[i*n+i] = 1.0

		vi := candidates[i].ContentVec
		if len(vi) != variables.VectorDimTotal {
			continue
		}

		for j := i + 1; j < n; j++ {
			vj := candidates[j].ContentVec
			if len(vj) != variables.VectorDimTotal {
				continue
			}

			// G[i][j] = G[j][i] = <ĉ_{p_i}, ĉ_{p_j}> (symétrie)
			dot := dotProductN(vi, vj)
			G[i*n+j] = dot
			G[j*n+i] = dot
		}
	}
	return G, n
}

// getOrBuildSimMatrix retourne la matrice de similarité depuis le cache_service ou la reconstruit.
//
// TDD §4.3:
//
//	"calculée une fois par fenêtre temporelle (toutes les 5 minutes) et mise en
//	 cache_service en mémoire dans le process Go"
//
// Invalidation: TTL de 5 minutes OU changement de l'ensemble de candidats.
func getOrBuildSimMatrix(candidates []PostCandidate) ([]float32, int) {
	now := time.Now()

	simCacheMu.RLock()
	if simCache != nil && now.Before(simCache.expiresAt) && candidatesMatch(simCache.postIDs, candidates) {
		G, n := simCache.G, simCache.n
		simCacheMu.RUnlock()
		return G, n
	}
	simCacheMu.RUnlock()

	// Reconstruction nécessaire (expiration ou ensemble de candidats modifié)
	G, n := buildSimilarityMatrix(candidates)

	postIDs := make([]int64, len(candidates))
	for i, c := range candidates {
		postIDs[i] = c.PostID
	}

	simCacheMu.Lock()
	// Double-check après acquisition du write lock (pattern DCLP)
	if simCache != nil && now.Before(simCache.expiresAt) && candidatesMatch(simCache.postIDs, candidates) {
		g, tn := simCache.G, simCache.n
		simCacheMu.Unlock()
		return g, tn
	}
	simCache = &simMatrixState{
		G:         G,
		n:         n,
		postIDs:   postIDs,
		expiresAt: now.Add(5 * time.Minute), // TDD §4.3: fenêtre de 5 minutes
	}
	simCacheMu.Unlock()

	return G, n
}

// candidatesMatch vérifie si la liste de post_service IDs correspond aux candidats actuels.
// Comparaison O(n) sur les int64, négligeable par rapport à la construction O(n²·N).
func candidatesMatch(cachedIDs []int64, candidates []PostCandidate) bool {
	if len(cachedIDs) != len(candidates) {
		return false
	}
	for i, c := range candidates {
		if cachedIDs[i] != c.PostID {
			return false
		}
	}
	return true
}

// ─────────────────────────────────────────────────────────────────────────────
// MMR — MAXIMAL MARGINAL RELEVANCE — TDD §4.3
// ─────────────────────────────────────────────────────────────────────────────

// RunMMR sélectionne itérativement les posts maximisant pertinence et diversité.
//
// TDD §4.3 — Formule itérative:
//
//	p*_i = argmax_{p ∈ C\S_i} [λ_d · R(u,p) - (1-λ_d) · max_{p'∈S_i} G[p][p']]
//
// Paramètres:
//   - candidates: posts candidats avec PersonalScore pré-calculé et MatrixIdx défini
//   - G: matrice de similarité aplatie [totalN×totalN] (G[i*totalN+j] = <ĉ_i,ĉ_j>)
//   - totalN: dimension de G (= len(candidates) pour matrice on-the-fly, ou |C| pour cache_service)
//   - lambdaD: paramètre de diversité (TDD: 0.72)
//   - k: nombre de posts à sélectionner (TDD: 50)
//
// Complexité: O(K · |C|) lookups dans G, soit O(50·1000) = O(50K) ≈ 2–5 ms (TDD §4.3).
func RunMMR(candidates []PostCandidate, G []float32, totalN int, lambdaD float64, k int) []PostCandidate {
	n := len(candidates)
	if n == 0 || k <= 0 {
		return nil
	}
	if k > n {
		k = n
	}

	selected := make([]PostCandidate, 0, k)
	selectedIdxs := make([]int, 0, k) // Indices dans candidates[] des posts sélectionnés
	available := make([]bool, n)
	for i := range available {
		available[i] = true
	}

	for len(selected) < k {
		bestMMR := math.Inf(-1)
		bestI := -1

		for i := range candidates {
			if !available[i] {
				continue
			}

			// ── Terme de pertinence: λ_d · R(u,p) ──────────────────────
			//
			// TDD §4.3: premier terme — favorise la pertinence personnalisée
			mmrScore := lambdaD * candidates[i].PersonalScore

			// ── Terme de redondance: (1-λ_d) · max_{p'∈S_i} G[p][p'] ──
			//
			// TDD §4.3: second terme — pénalise la similarité aux posts déjà sélectionnés
			if len(selectedIdxs) > 0 {
				var maxSim float64
				ci := candidates[i].MatrixIdx

				for _, selI := range selectedIdxs {
					cj := candidates[selI].MatrixIdx
					// G[ci][cj] = <ĉ_{p_i}, ĉ_{p_j}>
					if ci >= 0 && cj >= 0 && ci*totalN+cj < len(G) {
						sim := float64(G[ci*totalN+cj])
						if sim > maxSim {
							maxSim = sim
						}
					}
				}
				mmrScore -= (1.0 - lambdaD) * maxSim
			}

			if mmrScore > bestMMR {
				bestMMR = mmrScore
				bestI = i
			}
		}

		if bestI < 0 {
			break // Plus de candidats disponibles
		}

		// Ajout du candidat optimal à l'ensemble sélectionné
		selected = append(selected, candidates[bestI])
		selectedIdxs = append(selectedIdxs, bestI)
		available[bestI] = false
	}

	return selected
}

// ─────────────────────────────────────────────────────────────────────────────
// SÉRENDIPITÉ — TDD §4.3
// ─────────────────────────────────────────────────────────────────────────────

// InjectSerendipity remplace de manière ondulatoire des slots du feed par des posts de découverte.
func InjectSerendipity(feed []PostCandidate, pool []int64, rng *rand.Rand, startIndex int) []PostCandidate {
	if len(pool) == 0 || rng == nil {
		return feed
	}
	for i := range feed {
		// 1. Calcul de la Qualité Requise (Affinité) via la Vague de Dopamine
		// ✅ L'index global (startIndex + i) garantit la continuité parfaite de l'onde
		affinityRequired := DopamineWave(float64(startIndex + i))

		// 2. La probabilité d'injecter de la sérendipité (Exploration)
		// correspond au vide laissé par l'affinité.
		// Exemple : Si la vague exige 1.0 (Index 0), la probabilité est 0%.
		// Si la vague creuse un plateau à 0.35, la probabilité monte à 65%.
		probSerendipity := 1.0 - affinityRequired

		// 3. Tirage aléatoire déterministe via la Seed du panier
		if rng.Float64() < probSerendipity {
			poolIdx := rng.Intn(len(pool))
			feed[i] = PostCandidate{
				PostID:        pool[poolIdx],
				IsSerendipity: true,
				MatrixIdx:     -1, // Post hors matrice de similarité
			}
		}
	}
	return feed
}
