package variables

// ============================================================================
// PARAMÈTRES DE CONFIGURATION — ALGORITHMES DE RECOMMANDATION v1.0
// Source: TDD Algorithmes Recommandation, Section 6
//
// CONVENTION: Tout symbole préfixé TDD* est une valeur directement transcrite
// du document de conception. Toute modification requiert validation de l'équipe algo.
// ============================================================================

const (
	// ================================================================
	// PILIER 2 — SCORE DE TENDANCE S(p,t)
	// TDD §3.1 / §3.2 / §6
	// ================================================================

	// TDD §3.1 — Poids des signaux d'engagement
	// Formule: Σ_{s ∈ S} w_s · n_s(p),  S = {like, comment, media}
	TDDWeightLike    = 1.0 // w_like
	TDDWeightComment = 1.5 // w_comment
	TDDWeightMedia   = 0.3 // w_media — bonus de richesse du contenu

	// TDD §3.2 / §6 — Paramètres du score de base S_base (loi de puissance)
	// S_base(p,t) = (Σ w_s·n_s)^α / (Δt/3600 + θ)^β
	TDDAlpha = 0.85 // α — exposant sublinéaire (amortit la dominance virale, loi de Zipf)
	TDDBeta  = 1.60 // β — exposant du déclin temporel polynomial
	TDDTheta = 1.80 // θ — constante de gravité (≈108 min de période de grâce initiale)

	// TDD §3.2 / §6 — Déclin exponentiel D_exp
	// D_exp(p,t) = exp(-λ · max(0, Δt/3600 - T_grace))
	// Demi-vie effective post-grâce: t_{1/2} = ln(2)/λ = 0.693/0.04 ≈ 17.3 h
	TDDLambdaDecay = 0.04 // λ (h⁻¹) — taux de déclin exponentiel post-grâce
	TDDTGrace      = 72.0 // T_grace (h) — 72 heures de déclin purement polynomial

	// TDD §3.2 / §6 — Facteurs de qualité Φ(p) = Φ_media(p) · Φ_author(p) · Φ_mod(p)
	// Φ_media(p) = 1 + φ_m · I[|media_ids(p)| ≥ 1]
	TDDPhiMedia = 0.25 // φ_m — bonus média (+25%)
	// Φ_author(p) = 1 + φ_g · g_author / g_max
	TDDPhiGrade = 0.15 // φ_g — bonus grade auteur
	TDDGradeMax = 3.0  // g_max — grade maximum (modérateur)

	// TDD §3.2 / §6 — Facteur de diversité auteur V(p)
	// V(p) = exp(-γ_auth · k_author(p, W))
	// k=0→V=1.0 ; k=1→V≈0.607 ; k=2→V≈0.368
	TDDGammaAuth = 0.50 // γ_auth — taux de pénalité diversité auteur
)
