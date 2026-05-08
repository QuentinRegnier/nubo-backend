package service

import (
	"math"

	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// PILIER 2 — ALGORITHME DE RECOMMANDATION GLOBAL (DÉTECTION DES TENDANCES)
// TDD §3.1, §3.2, §3.3, §3.4
// ============================================================================

// ScoreOptions contient toutes les métriques nécessaires au calcul de S(p,t).
//
// RÉTROCOMPATIBILITÉ: Les champs BoostLikes/BoostComments/BoostRecent sont
// conservés. Ils agissent comme des multiplicateurs optionnels sur les poids
// TDD fixes. Une valeur ≤ 1.0 (ou zéro) reproduit exactement le comportement TDD.
//
// NOUVEAU (TDD §3.2): IsDeleted et IsReported implémentent Φ_mod(p).
// Ces champs ont pour valeur par défaut false → Φ_mod = 1.0 (comportement neutre).
type ScoreOptions struct {
	// ── Signaux d'engagement (TDD §3.1) ─────────────────────────────────
	LikesCount    int // n_like(p)
	CommentsCount int // n_comment(p)
	MediaCount    int // n_media(p) = |media_ids(p)|

	// ── Contexte auteur (TDD §3.2) ───────────────────────────────────────
	AuthorGrade         int // g_author ∈ {0,1,2,3}
	AuthorPostsInWindow int // k_author(p,W) — posts du même auteur dans le ZSET courant

	// ── Contexte temporel ────────────────────────────────────────────────
	AgeSeconds float64 // Δt = t - t_p (secondes)

	// ── Modulateurs hérités (DEPRECATED — conservés pour rétrocompatibilité) ─
	// Multiplient les poids TDD fixes. math.Max(1.0, x) garantit qu'ils ne
	// réduisent jamais les poids en dessous de leurs valeurs TDD.
	BoostLikes    float64
	BoostComments float64
	BoostRecent   float64 // Réduit β si > 1.0 (ralentit la gravité temporelle)

	// ── Facteur de modération Φ_mod — NOUVEAU (TDD §3.2) ─────────────────
	// IsDeleted: post supprimé (visibility=0) → Φ_mod = 0 → S = 0.0 immédiatement.
	IsDeleted bool
	// IsReported: signalé en attente de modération → Φ_mod = 0.5.
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

	// ────────────────────────────────────────────────────────────────────
	// Composante 0 — Φ_mod(p) : Facteur de modération
	//
	// TDD §3.2:
	//   Φ_mod = 0   si visibility = 0 (supprimé, conservation légale)
	//   Φ_mod = 1   si visibility ∈ {1, 2} (public ou abonnés)
	//   Φ_mod = 0.5 si signalé en attente de modération
	//
	// Court-circuit : les posts supprimés ne doivent jamais être indexés.
	// ────────────────────────────────────────────────────────────────────
	if opts.IsDeleted {
		return 0.0
	}
	var phiMod float64
	if opts.IsReported {
		phiMod = 0.5
	} else {
		phiMod = 1.0
	}

	// ────────────────────────────────────────────────────────────────────
	// Composante 1a — Somme pondérée des signaux d'engagement
	//
	// TDD §3.1:
	//   Σ_{s ∈ S} w_s · n_s(p)
	//   w_like = 1.0,  w_comment = 1.5,  w_media = 0.3
	//
	// Les boosts hérités multiplient les poids TDD. math.Max(1.0, ·)
	// garantit que les boosts ne diminuent jamais les poids.
	// ────────────────────────────────────────────────────────────────────
	wLike := variables.TDDWeightLike * math.Max(1.0, opts.BoostLikes)
	wComment := variables.TDDWeightComment * math.Max(1.0, opts.BoostComments)
	wMedia := variables.TDDWeightMedia

	engagementSum := wLike*float64(opts.LikesCount) +
		wComment*float64(opts.CommentsCount) +
		wMedia*float64(opts.MediaCount)

	// Optimisation CPU: si aucun engagement, S = 0 (évite math.Pow coûteux).
	if engagementSum <= 0 {
		return 0.0
	}

	// ────────────────────────────────────────────────────────────────────
	// Composante 1b — S_base(p, t) : Score de base (loi de puissance)
	//
	// TDD §3.2:
	//   S_base(p,t) = (Σ w_s·n_s(p))^α / (Δt/3600 + θ)^β
	//   α = 0.85  (sublinéaire : amortit la dominance virale)
	//   β = 1.60  (exposant de déclin temporel polynomial)
	//   θ = 1.80  (≈108 min de période de grâce initiale)
	// ────────────────────────────────────────────────────────────────────
	ageHours := math.Max(0.0, opts.AgeSeconds/3600.0)

	// BoostRecent hérité : divise β pour ralentir la gravité si > 1.0.
	beta := variables.TDDBeta
	if opts.BoostRecent > 1.0 {
		beta = beta / opts.BoostRecent
	}

	numerator := math.Pow(engagementSum, variables.TDDAlpha)
	denominator := math.Pow(ageHours+variables.TDDTheta, beta)
	sBase := numerator / denominator

	// ────────────────────────────────────────────────────────────────────
	// Composante 2 — D_exp(p, t) : Déclin exponentiel post-grâce
	//
	// TDD §3.2:
	//   D_exp(p,t) = exp(-λ · max(0, Δt/3600 - T_grace))
	//   λ = 0.04 h⁻¹,  T_grace = 72 h
	//   t_{1/2} = ln(2)/0.04 ≈ 17.3 h (demi-vie effective post-grâce)
	//
	// Pendant 72 h: D_exp = 1.0 (déclin purement via S_base).
	// Après 72 h: déclin exponentiel supplémentaire → convergence vers 0 garantie.
	// ────────────────────────────────────────────────────────────────────
	dExp := 1.0
	if ageHours > variables.TDDTGrace {
		dExp = math.Exp(-variables.TDDLambdaDecay * (ageHours - variables.TDDTGrace))
	}

	// ────────────────────────────────────────────────────────────────────
	// Composante 3 — Φ(p) : Facteur de qualité du contenu
	//
	// TDD §3.2:
	//   Φ(p) = Φ_media(p) · Φ_author(p) · Φ_mod(p)
	// ────────────────────────────────────────────────────────────────────

	// Φ_media(p) = 1 + φ_m · I[|media_ids(p)| ≥ 1]
	// φ_m = 0.25 → posts avec médias : boost de +25%
	phiMedia := 1.0
	if opts.MediaCount > 0 {
		phiMedia = 1.0 + variables.TDDPhiMedia // = 1.25
	}

	// Φ_author(p) = 1 + φ_g · g_author / g_max
	// φ_g = 0.15,  g_max = 3
	// g=0→1.00 ; g=1→1.05 ; g=2→1.10 ; g=3→1.15
	gradeRatio := math.Min(1.0, float64(opts.AuthorGrade)/variables.TDDGradeMax)
	phiAuthor := 1.0 + variables.TDDPhiGrade*gradeRatio

	// Φ(p) = Φ_media · Φ_author · Φ_mod
	phi := phiMedia * phiAuthor * phiMod

	// ────────────────────────────────────────────────────────────────────
	// Composante 4 — V(p) : Facteur de diversité auteur
	//
	// TDD §3.2:
	//   V(p) = exp(-γ_auth · k_author(p, W))
	//   γ_auth = 0.50
	//   k_author = nombre de posts du même auteur déjà dans la fenêtre W (ZSET)
	// ────────────────────────────────────────────────────────────────────
	vFactor := math.Exp(-variables.TDDGammaAuth * float64(opts.AuthorPostsInWindow))

	// ────────────────────────────────────────────────────────────────────
	// Formule composite finale
	//
	// TDD §3.2:
	//   S(p,t) = S_base(p,t) · D_exp(p,t) · Φ(p) · V(p)
	// ────────────────────────────────────────────────────────────────────
	return sBase * dExp * phi * vFactor
}
