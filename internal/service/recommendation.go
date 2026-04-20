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

// CalculateRecommendationScore génère le score algorithmique d'un post.
// Pour le moment, renvoie une valeur aléatoire en attendant l'implémentation finale.
//
// Paramètres :
// - postID : L'identifiant du post à évaluer.
// - bonuses : Un tableau de modificateurs (ex: [500.0, 150.0]) pour buffer le score. Inutilisé pour le moment.
func CalculateRecommendationScore(postID int64, bonuses []float64) float64 {
	// TODO: Implémenter le vrai calcul d'engagement et de Time Decay plus tard.

	// Pour le moment : Génération d'un score aléatoire entre 0 et 10000
	// Cela permet de tester le comportement du ZSET (classement, capping)
	// sans avoir l'algorithme définitif.
	r := rand.New(rand.NewSource(time.Now().UnixNano() + postID))
	randomScore := r.Float64() * 10000.0

	return randomScore
}
