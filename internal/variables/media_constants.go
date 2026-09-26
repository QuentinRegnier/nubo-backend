package variables

// ============================================================================
// PARAMÈTRES ET CONSTANTES DU DOMAINE MÉDIA
// ============================================================================

const (
	// Limites de résolution pour les uploads d'images
	MediaMaxPixels = 2000 * 2000 // Limite de surface (4 Mégapixels)
	MediaMaxWidth  = 1920        // Largeur maximale avant redimensionnement (Lanczos)

	// Paramètres d'encodage AVIF
	MediaAvifQuality = 65 // Qualité de compression (0-100) : 65 offre un excellent ratio poids/qualité
	MediaAvifSpeed   = 5  // Vitesse d'encodage (0-10) : 5 est le meilleur compromis CPU/Temps
)
