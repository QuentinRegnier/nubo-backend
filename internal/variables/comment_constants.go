package variables

// ============================================================================
// PARAMÈTRES ET CONSTANTES DU DOMAINE COMMENTAIRES
// ============================================================================

const (
	// CommentPriorityMultiplier est le coefficient appliqué au grade de l'auteur (0 à 4)
	// pour calculer le score initial d'affichage dans le ZSET des commentaires (L1).
	// Valeur d'origine extraite du code en dur : 10000.
	// Exemple : Admin (Grade 4) -> 4 * 10000 = Score initial de 40000.
	CommentPriorityMultiplier = 10000

	CommentVisibilitySoftDelete = -1
)
