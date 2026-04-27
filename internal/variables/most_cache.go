package variables

// Poids (Boosts) de l'algorithme de recommandation (Time-Decay)
const (
	BoostRecent = 0.30 // +30% pour le flux "rank:recent:global"
	BoostLikes  = 0.50 // +50% pour le flux "rank:likes:global"
	BoostViews  = 0.20 // +20% pour le flux "rank:views:global"
)

// Limites de cache pour les flux "Most" (Top) et les posts d'un utilisateur
const (
	MaxTagElements       = 5000 // On ne garde que les 5000 posts les plus hauts par tag
	MaxRankElements      = 5000 // On ne garde que le top 5000 (Likes, Vues, Global, Recent)
	MaxUserPostsElements = 100  // On ne garde que les 100 derniers posts d'un utilisateur en RAM
)
