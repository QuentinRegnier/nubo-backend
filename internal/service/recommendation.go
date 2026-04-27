package service

import (
	"math/rand"
	"time"
)

// ============================================================================
// MOTEUR DE RECOMMANDATION & SCORING (ALGORITHME)
// ============================================================================
//
// 🧠 IDÉE DERRIÈRE L'ALGORITHME (À implémenter plus tard) :
//
// Le but de cette fonction est de calculer un score de "Chaleur" (Hotness) pour un post.
// Si on se contente de trier par "Nombre de Likes", les vieux posts extrêmement
// populaires resteront toujours premiers et écraseront les nouveaux contenus.
//
// Pour éviter ça, l'algorithme final devra utiliser 3 composantes :
//
// 1. LE SCORE DE BASE (Engagement) :
//    Chaque interaction a un poids différent.
//    Ex: (Vues * 1) + (Likes * 5) + (Commentaires * 10) + (Sauvegardes * 15)
//
// 2. LES BONUS (Modulateurs contextuels) :
//    Le tableau `bonuses` permet d'injecter des boosts artificiels ponctuels.
//    Exemple de bonus possibles :
//    - L'auteur est un nouvel utilisateur (Boost de visibilité : +500 pts).
//    - Le post contient une image de haute qualité (Boost qualité : +200 pts).
//    - Le post traite d'un sujet ultra-tendance aujourd'hui (Boost trend : +300 pts).
//
// 3. LE TIME DECAY (La gravité temporelle) :
//    C'est le secret algorithmique (similaire à Reddit ou HackerNews).
//    Le score DOIT baisser avec le temps pour laisser la place aux jeunes.
//    Formule type HackerNews : Score = (EngagementBase + Bonus) / (AgeEnHeures + 2)^Gravité
//    Plus le temps passe, plus le diviseur est grand, donc le score chute.
//    Ainsi, un post tout neuf avec 10 likes peut temporairement battre un post
//    de la semaine dernière avec 10 000 likes.
//
// ============================================================================

// ScoreOptions permet de moduler le calcul du score algorithmique
// en appliquant des boosts spécifiques (équivalent des arguments par défaut).
// Une valeur non renseignée lors de l'appel vaudra automatiquement 0.0 en Go.
type ScoreOptions struct {
	BoostLikes  float64
	BoostViews  float64
	BoostRecent float64
	// D'autres boosts pourront être ajoutés ici (BoostComments, BoostQuality...)
}

// CalculateRecommendationScore génère le score algorithmique d'un post.
func CalculateRecommendationScore(postID int64, opts ScoreOptions) float64 {
	// TODO: Implémenter le vrai calcul d'engagement et de Time-Decay plus tard.
	// Cette fonction utilisera "opts.BoostLikes", "opts.BoostViews", etc.

	// L'intérieur reste volontairement factice pour le moment :
	r := rand.New(rand.NewSource(time.Now().UnixNano() + postID))
	randomScore := r.Float64() * 10000.0

	// On utilise la variable opts de manière fantôme pour éviter l'erreur de compilation Go "declared but not used"
	_ = opts.BoostLikes + opts.BoostViews + opts.BoostRecent

	return randomScore
}
