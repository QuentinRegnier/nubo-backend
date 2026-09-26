package variables

import "time"

const (
	SocialRatio = 0.3 // 30% d'abonnements/amis
	TagRatio    = 0.5 // 50% d'affinités thématiques (Graphe 1-Hop)
	GlobalRatio = 0.2 // 20% de sérendipité mondiale pure
)

// Ces variables pourront être migrées vers Redis plus tard pour être modifiées à chaud (sans redémarrer le serveur).
const (
	FeedReloadDelay = 30 * time.Minute // Temps avant d'autoriser une vraie nouvelle génération
	FeedPageSize    = 50               // Nombre de posts renvoyés par scroll
)

// ============================================================================
// CONSTANTES SPÉCIFIQUES AU WORKER FEED CACHE
// ============================================================================
const (
	FeedWarmupCronInterval = 5 * time.Minute // Fréquence d'analyse des flux expirés
	FeedWarmupBatchSize    = 500             // Nombre maximum d'utilisateurs réhydratés par cycle
	FanOutVIPThreshold     = 50000           // Limite d'abonnés déclenchant le coupe-circuit "Justin Bieber"
	FanOutRetentionLimit   = -501            // Purge ZSET (ZREMRANGEBYRANK) pour conserver les 500 derniers posts
)
