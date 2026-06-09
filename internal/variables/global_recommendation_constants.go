package variables

// ============================================================================
// PARAMÈTRES DE CONFIGURATION — ALGORITHMES DE RECOMMANDATION
// (La "Table de mixage" - Fusionnée depuis l'ancien TDD)
// ============================================================================

const (

	// --- PILIER 3 : SCORE PERSONNALISÉ R(u,p) ---
	TDDRho  = 0.65 // ρ — poids composante vectorielle vs sociale
	TDDEta  = 0.20 // η — boost amis directs
	TDDEtaP = 0.10 // η_P — poids corrélation de Pearson engagement

	TDDLambdaMMR = 0.72 // λ_d — paramètre de diversité MMR

	TDDDeltaInvalid = 0.15 // δ_inval — seuil d'invalidation du cache_service feed_service

	TDDLSHBits = 32        // b — bits de projection aléatoire (SimHash)
	TDDLSHSeed = int64(42) // Graine LSH
)
