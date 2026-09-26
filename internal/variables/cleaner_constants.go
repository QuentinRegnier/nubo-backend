package variables

import "time"

// ============================================================================
// CONSTANTES SPÉCIFIQUES AU WORKER GARBAGE COLLECTOR DES LIKES
// ============================================================================
const (
	LikeCleanupCronInterval = 6 * time.Hour // Fréquence de nettoyage (Toutes les 6h)
)

// ============================================================================
// CONSTANTES SPÉCIFIQUES AU WORKER GARBAGE COLLECTOR DES MÉDIAS
// ============================================================================
const (
	MediaCleanupCronInterval = 1 * time.Hour // Fréquence de nettoyage des fichiers orphelins
)

// ============================================================================
// CONSTANTES SPÉCIFIQUES AU WORKER GARBAGE COLLECTOR DES RÉACTIONS
// ============================================================================
const (
	ReactionCleanupCronInterval = 6 * time.Hour // Fréquence de nettoyage (Toutes les 6h)
)

// ============================================================================
// CONSTANTES SPÉCIFIQUES AU WORKER GARBAGE COLLECTOR DES FAVORIS
// ============================================================================
const (
	SavedCleanupCronInterval = 6 * time.Hour // Fréquence de nettoyage (Toutes les 6h)
)
