package variables

// ============================================================================
// LIMITES OPÉRATIONNELLES ET CACHE
// ============================================================================
const (
	MaxStrictElements = 5000 // Remplace MaxRankElements — Top absolu pour l'UI (Likes, Vues)
	MaxTagElements    = 5000 // Taille max pour l'historique brut d'un tag
)

// ============================================================================
// REDIS KEYS - NOMENCLATURE UNIFIÉE
// ============================================================================
const (

	// 4. Hashtags & Canonicalisation
	RedisKeyActiveTagsSet = "most_cache:tags:active:set"

	// 5. Vecteur de Contenu (Object Cache car donnée pure JSON/MsgPack)
	RedisKeyContentVector = "object_cache:content:vec:%d"

	// ================================================================
	// LIMITES OPÉRATIONNELLES — TDD §6
	// ================================================================
	TDDMaxZSET    = 500  // MAX_ZSET — taille max par ZSET de tendance
	TDDCandidates = 1000 // |C| — taille de l'ensemble de candidats pour le feed_service
	TDDFeedSize   = 50   // K_feed — taille du feed_service personnalisé retourné

	// ================================================================
	// CLÉS REDIS — TDD §4.4
	// ================================================================
	// Note: RedisKeyContentVector = "content:vec:%d" est conservé dans constant.go
	RedisKeyFeedPersonalized  = "feed_service:personalized:%d"
	RedisKeyTrendGlobalHourly = "most_cache:trend:global:hourly:%s"
	RedisKeyTrendGlobalDaily  = "most_cache:trend:global:daily:%s"
	// On ajoute :%s à la fin pour injecter la date (YYYYMMDD) ou la semaine (YYYY-WXX)
	RedisKeyTrendTagDaily      = "most_cache:trend:tag:%s:daily:%s"
	RedisKeyTrendTagWeekly     = "most_cache:trend:tag:%s:weekly:%s"
	RedisKeyHashtagLeaderboard = "most_cache:trend:hashtag:leaderboard"
	RedisKeyHashtagCanonMap    = "most_cache:hashtag:canon:map"
	RedisKeyLSHBucket          = "most_cache:lsh:bucket:%d"

	// 2. Classements Stricts (Pour l'interface utilisateur uniquement)
	RedisKeyStrictLikes  = "most_cache:strict:likes"
	RedisKeyStrictViews  = "most_cache:strict:views"
	RedisKeyStrictRecent = "most_cache:strict:recent"

	// TDD §4.1 — Paramètre gaussien du bloc temporel
	// c_{p,k}^(temp) = exp(-(k - h_p)² / (2·σ_h²)) · Z^{-1}
	TDDSigmaHours         = 2.0  // σ_h (h) — lissage gaussien sur l'heure
	TDDWeightView         = 0.1  // w_view — Poids d'une vue pure
	TDDPhiReported        = 0.5  // φ_mod — Pénalité si le post_service est signalé
	TDDHashtagWindowHours = 48.0 // Fenêtre de validité pour les trends hashtags
)
