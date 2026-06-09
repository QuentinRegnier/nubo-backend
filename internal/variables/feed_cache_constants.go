package variables

import "time"

// Ces variables pourront être migrées vers Redis plus tard pour être modifiées à chaud (sans redémarrer le serveur).
const (
	FeedReloadDelay = 30 * time.Minute // Temps avant d'autoriser une vraie nouvelle génération
	FeedPageSize    = 50               // Nombre de posts renvoyés par scroll
)
