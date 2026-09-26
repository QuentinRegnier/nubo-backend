package variables

import "time"

// ============================================================================
// CONSTANTES SPÉCIFIQUES AU WORKER D'INTERACTIONS (MESSAGERIE)
// ============================================================================
const (
	InteractionBufferSize     = 50000           // Capacité maximale du Channel en RAM
	InteractionBatchThreshold = 5000            // Seuil de déclenchement du vidage anticipé (Viralité)
	InteractionFlushInterval  = 5 * time.Second // Intervalle de vidage automatique du tampon
)
