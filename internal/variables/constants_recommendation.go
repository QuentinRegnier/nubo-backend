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

	// ================================================================
	// PILIER 3 — SCORE PERSONNALISÉ R(u,p)
	// TDD §4.2 / §4.3 / §4.5 / §6
	// ================================================================

	// TDD §4.2 — Pondérations du score personnalisé
	// R(u,p) = S(p,t)·[ρ·<û,ĉ_p> + (1-ρ)·A(u,p) + η·B(u,p) + η_P·r_Pearson(u,p)]
	TDDRho  = 0.65 // ρ — poids composante vectorielle vs. sociale
	TDDEta  = 0.20 // η — boost binaire posts d'amis directs B(u,p) ∈ {0,1}
	TDDEtaP = 0.10 // η_P — poids corrélation de Pearson sur l'engagement

	// TDD §4.3 / §6 — MMR (Maximal Marginal Relevance)
	// p*_i = argmax_{p ∈ C\S_i}[λ_d·R(u,p) - (1-λ_d)·max_{p'∈S_i}<ĉ_p, ĉ_p'>]
	TDDLambdaMMR   = 0.72 // λ_d — paramètre de diversité (1.0=pur relevance, 0.0=pur diversité)
	TDDSerendipity = 0.08 // p_serendip — probabilité d'injection sérendipité par slot

	// TDD §4.4 — Cache et invalidation
	TDDDeltaInvalid = 0.15 // δ_inval — seuil ||û_{t+1} - û_t||_2 pour invalider le cache feed
	TDDFeedCacheTTL = 300  // TTL du cache feed (s) = 5 minutes

	// TDD §4.5 — LSH Random Projections (SimHash)
	// LSH(v) = bin(sign(P·v)) ∈ {0,1}^b,  P ∈ R^{b×224}
	// Pr[LSH(u)=LSH(c_p)] = 1 - arccos(<û,ĉ_p>)/π
	TDDLSHBits = 32        // b — nombre de bits de projection aléatoire
	TDDLSHSeed = int64(42) // graine fixe pour la reproductibilité (distribuée à tous les workers)

	// ================================================================
	// DIMENSIONS VECTORIELLES — TDD §2.2 / §6
	// u = [u^(cat) | u^(temp) | u^(eng) | u^(soc)] ∈ R^224
	// ================================================================
	VectorDimTotal = 224 // N — dimension totale
	VectorDimCat   = 128 // N_cat — catégoriel SVD hashtags    [0:128)
	VectorDimTemp  = 24  // N_temp — temporel heure journalière [128:152)
	VectorDimEng   = 8   // N_eng  — engagement comportemental  [152:160)
	VectorDimSoc   = 64  // N_soc  — graphe social              [160:224)

	// Offsets de début de chaque bloc dans le vecteur complet
	VectorOffCat  = 0   // début bloc catégoriel
	VectorOffTemp = 128 // début bloc temporel
	VectorOffEng  = 152 // début bloc engagement
	VectorOffSoc  = 160 // début bloc social

	// TDD §4.1 — Paramètre gaussien du bloc temporel
	// c_{p,k}^(temp) = exp(-(k - h_p)² / (2·σ_h²)) · Z^{-1}
	TDDSigmaHours = 2.0 // σ_h (h) — écart-type du lissage gaussien sur l'heure

	// ================================================================
	// LIMITES OPÉRATIONNELLES — TDD §6
	// ================================================================
	TDDMaxZSET    = 500  // MAX_ZSET — taille max par ZSET de tendance
	TDDCandidates = 1000 // |C| — taille de l'ensemble de candidats pour le feed
	TDDFeedSize   = 50   // K_feed — taille du feed personnalisé retourné

	// ================================================================
	// CLÉS REDIS — TDD §4.4
	// ================================================================
	// Note: RedisKeyContentVector = "content:vec:%d" est conservé dans constant.go
	RedisKeyUserProfile        = "user:profile:%d"
	RedisKeyFeedPersonalized   = "feed:personalized:%d"
	RedisKeyTrendGlobalHourly  = "trend:global:hourly:%s"
	RedisKeyTrendGlobalDaily   = "trend:global:daily:%s"
	RedisKeyTrendTagDaily      = "trend:tag:%s:daily"
	RedisKeyTrendTagWeekly     = "trend:tag:%s:weekly"
	RedisKeyHashtagLeaderboard = "trend:hashtag:leaderboard"
	RedisKeyHashtagCanonMap    = "hashtag:canon:map"
	RedisKeyLSHBucket          = "lsh:bucket:%d"
)
