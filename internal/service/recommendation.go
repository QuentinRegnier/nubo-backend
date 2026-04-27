package service

import (
	"math"
	"time"
)

// ============================================================================
// MOTEUR DE RECOMMANDATION GLOBAL & SCORING (ALGORITHME V1.0)
// Implémente le calcul des flux "rank:*:global"
// ============================================================================

// ScoreOptions contient les métriques brutes et les modulateurs de boost
// pour générer les différents ZSETs (standard, likes, comments, recent).
type ScoreOptions struct {
	LikesCount          int
	CommentsCount       int
	MediaCount          int
	AuthorGrade         int
	AgeSeconds          float64
	AuthorPostsInWindow int

	// --- Modulateurs (Boosts) ---
	// Une valeur <= 1.0 signifie aucun boost.
	// Exemple : 1.5 ajoute 50% de puissance au critère ciblé.
	BoostLikes    float64
	BoostComments float64
	BoostRecent   float64
}

// CalculateRecommendationScore génère le score algorithmique global d'un post S(p,t).
func CalculateRecommendationScore(postID int64, opts ScoreOptions) float64 {
	// 1. Poids des signaux d'engagement avec application des Boosts
	wLike := 1.0 * math.Max(1.0, opts.BoostLikes)
	wComment := 1.5 * math.Max(1.0, opts.BoostComments)
	wMedia := 0.3

	engagementSum := (float64(opts.LikesCount) * wLike) +
		(float64(opts.CommentsCount) * wComment) +
		(float64(opts.MediaCount) * wMedia)

	// Si aucun engagement et pas de boost fraîcheur massif, score = 0 (optimisation CPU)
	if engagementSum <= 0 {
		return 0.0
	}

	// 2. Score de base (Loi de puissance sur l'engagement et déclin polynomial)
	const alpha = 0.85 // Exposant sublinéaire (amortit la viralité)

	// Le BoostRecent agit comme un bouclier anti-gravité
	// Plus il est grand, plus beta (le déclin) est faible.
	beta := 1.60
	if opts.BoostRecent > 1.0 {
		beta = beta / opts.BoostRecent
	}

	const theta = 1.80 // Période de grâce en heures

	ageHours := opts.AgeSeconds / 3600.0
	if ageHours < 0 {
		ageHours = 0
	}

	numerator := math.Pow(engagementSum, alpha)
	denominator := math.Pow(ageHours+theta, beta)
	sBase := numerator / denominator

	// 3. Déclin exponentiel post-période de grâce (s'applique à tous les flux)
	const lambda = 0.04
	const tGrace = 72.0

	dExp := 1.0
	if ageHours > tGrace {
		dExp = math.Exp(-lambda * (ageHours - tGrace))
	}

	// 4. Facteurs de Qualité du Contenu (Phi)
	phiMedia := 1.0
	if opts.MediaCount > 0 {
		phiMedia = 1.25 // Boost media de 25%
	}

	gradeRatio := float64(opts.AuthorGrade) / 3.0
	if gradeRatio > 1.0 {
		gradeRatio = 1.0
	}
	phiAuthor := 1.0 + (0.15 * gradeRatio)
	phi := phiMedia * phiAuthor

	// 5. Facteur de diversité d'auteur (V)
	const gammaAuth = 0.50
	vFactor := math.Exp(-gammaAuth * float64(opts.AuthorPostsInWindow))

	// Application de la formule composite finale
	return sBase * dExp * phi * vFactor
}

// ============================================================================
// VECTORISATION DE CONTENU (PILIER 3 - RECOMMANDATION PERSONNALISÉE)
// ============================================================================

// GenerateContentVector crée le vecteur mathématique c_p d'un post (224 dimensions)
// tel que spécifié dans le Document de Conception (Section 4.1).
func GenerateContentVector(createdAt time.Time, hashtags []string, authorID int64) []float32 {
	vector := make([]float32, 224)

	// Bloc 1: Catégoriel [0:127] (128 dimensions)
	// TODO: Requiert la matrice d'embedding SVD des tags canoniques. Laissé à 0 pour l'instant.

	// Bloc 2: Temporel [128:151] (24 dimensions)
	// Vecteur one-hot soft sur l'heure de publication avec lissage gaussien (sigma = 2h)
	hour := createdAt.Hour()
	const sigmaH = 2.0
	var sumZ float64

	for k := 0; k < 24; k++ {
		diff := float64(k - hour)
		// Formule: exp( -(k - h_p)^2 / (2 * sigma_h^2) )
		val := math.Exp(-(diff * diff) / (2 * sigmaH * sigmaH))
		vector[128+k] = float32(val)
		sumZ += val
	}

	// Normalisation Z^-1 du bloc temporel
	if sumZ > 0 {
		for k := 0; k < 24; k++ {
			vector[128+k] /= float32(sumZ)
		}
	}

	// Bloc 3: Engagement [152:159] (8 dimensions)
	// Initialement à 0 à la création (taux de like, coms, scroll = 0)

	// Bloc 4: Social [160:223] (64 dimensions)
	// TODO: Requiert l'embedding du graphe social de l'auteur. Laissé à 0 pour l'instant.

	return vector
}
