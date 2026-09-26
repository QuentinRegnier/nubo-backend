package variables

// ############################################################################
// # PARAMÈTRES ET CONSTANTES DU GRAPHE DE MARKOV (SÉMANTIQUE)
// ############################################################################

const (
	GraphDecayLambda   = 0.05  // λ : Taux de perte de pertinence (Ex: 5% d'oubli par jour)
	GraphLearningAlpha = 0.20  // α : Force d'apprentissage d'une nouvelle co-occurrence (20%)
	GraphSurvivalEps   = 0.001 // ε : Seuil d'oubli absolu (En dessous, le segment est détruit)
)

// ============================================================================
// CONSTANTES SPÉCIFIQUES AU WORKER DE GRAPHE SÉMANTIQUE
// ============================================================================
const (
	GraphTagMinLength  = 5       // Longueur minimale d'un tag pour être considéré comme sémantique
	GraphUserTagPrefix = "user_" // Préfixe utilisé pour les mentions/identifications directes
)
