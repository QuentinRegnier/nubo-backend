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
