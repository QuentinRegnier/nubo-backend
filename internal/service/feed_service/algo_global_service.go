package feed_service

import (
	"math"

	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// PILIER 2 — ALGORITHME DE RECOMMANDATION GLOBAL (DÉTECTION DES TENDANCES)
// TDD §3.1, §3.2, §3.3, §3.4
// ============================================================================

// ScoreOptions contient les métriques brutes nécessaires au calcul de S(p,t).
type ScoreOptions struct {
	// ── Signaux d'engagement (TDD §3.1) ─────────────────────────────────
	LikesCount    int // n_like(p)
	CommentsCount int // n_comment(p)
	ViewCount     int // n_view(p)
	MediaCount    int // n_media(p) = |media_ids(p)|

	// ── Contexte auteur (TDD §3.2) ───────────────────────────────────────
	AuthorGrade         int // g_author ∈ {0,1,2,3}
	AuthorPostsInWindow int // k_author(p,W) — posts du même auteur dans le ZSET courant

	// ── Contexte temporel ────────────────────────────────────────────────
	AgeSeconds float64 // Δt = t - t_p (secondes)

	// ── Facteur de modération Φ_mod (TDD §3.2) ───────────────────────────
	IsDeleted  bool
	IsReported bool
}

// CalculateRecommendationScore calcule S(p, t) — le score de tendance global.
//
// TDD §3.2 — Formule composite complète:
//
//	S(p, t) = S_base(p, t) · D_exp(p, t) · Φ(p) · V(p)
//
// Complexité: O(1) — opérations scalaires uniquement.
func CalculateRecommendationScore(_ int64, opts ScoreOptions) float64 {

	// 1. Facteur de modération Φ_mod(p)
	if opts.IsDeleted {
		return 0.0
	}
	var phiMod float64
	if opts.IsReported {
		phiMod = variables.TDDPhiReported
	} else {
		phiMod = 1.0
	}

	// 2. Somme pondérée des signaux d'engagement (Σ w_s · n_s(p))
	engagementSum := variables.TDDWeightLike*float64(opts.LikesCount) +
		variables.TDDWeightComment*float64(opts.CommentsCount) +
		variables.TDDWeightView*float64(opts.ViewCount) +
		variables.TDDWeightMedia*float64(opts.MediaCount)

	if engagementSum <= 0 {
		return 0.0
	}

	// 3. S_base(p, t) : Score de base avec déclin polynomial
	ageHours := math.Max(0.0, opts.AgeSeconds/3600.0)
	numerator := math.Pow(engagementSum, variables.TDDAlpha)
	denominator := math.Pow(ageHours+variables.TDDTheta, variables.TDDBeta)
	sBase := numerator / denominator

	// 4. D_exp(p, t) : Déclin exponentiel post_service-grâce
	dExp := 1.0
	if ageHours > variables.TDDTGrace {
		dExp = math.Exp(-variables.TDDLambdaDecay * (ageHours - variables.TDDTGrace))
	}

	// 5. Φ(p) : Facteur de qualité du contenu
	phiMedia := 1.0
	if opts.MediaCount > 0 {
		phiMedia = 1.0 + variables.TDDPhiMedia
	}

	gradeRatio := math.Min(1.0, float64(opts.AuthorGrade)/variables.TDDGradeMax)
	phiAuthor := 1.0 + variables.TDDPhiGrade*gradeRatio

	phi := phiMedia * phiAuthor * phiMod

	// 6. V(p) : Facteur de diversité auteur
	vFactor := math.Exp(-variables.TDDGammaAuth * float64(opts.AuthorPostsInWindow))

	// Formule finale
	return sBase * dExp * phi * vFactor
}

// ============================================================================
// CALCUL DES TENDANCES DE HASHTAGS (TDD §3.3)
// ============================================================================

// ComputeHashtagTrendScore calcule T(h, t) — le score de tendance d'un hashtag canonique.
//
// TDD §3.3 — Formule:
//
//	T(h, t) = Σ_{p ∈ P_h} S(p,t) · I[Δt(p) ≤ 48·3600]
//
// Paramètres:
//   - postScores: map[postID → S(p,t)] pour les posts contenant le hashtag h
//   - postAges: map[postID → Δt(p) en secondes]
func ComputeHashtagTrendScore(postScores map[int64]float64, postAges map[int64]float64) float64 {
	// Utilisation de la constante paramétrable depuis la table de mixage
	maxAgeSecs := variables.TDDHashtagWindowHours * 3600.0
	var totalScore float64

	for postID, score := range postScores {
		age, exists := postAges[postID]
		// Fonction indicatrice I : on n'additionne que si le post_service est dans la fenêtre temporelle
		if exists && age <= maxAgeSecs {
			totalScore += score
		}
	}

	return totalScore
}
