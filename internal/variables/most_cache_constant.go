package variables

// ---------------------------------------------------------
// REDIS KEYS - RECOMMANDATION & TENDANCES (ZSETS)
// ---------------------------------------------------------
const (
	// Mode Strict
	RedisKeyRankLikesStrict  = "most_cache:rank:likes:strict"
	RedisKeyRankViewsStrict  = "most_cache:rank:views:strict"
	RedisKeyRankRecentStrict = "most_cache:rank:recent:strict"

	// Mode Global
	RedisKeyRankGlobal         = "most_cache:rank:global"
	RedisKeyRankLikesGlobal    = "most_cache:rank:likes:global"
	RedisKeyRankCommentsGlobal = "most_cache:rank:comments:global"
	RedisKeyRankRecentGlobal   = "most_cache:rank:recent:global"

	// Tags & Perso
	RedisKeyRankTag       = "most_cache:idx:tag:%s"
	RedisKeyContentVector = "object_cache:content:vec:%d" // Le vecteur est de la data pure, donc OBJECT
)

// ---------------------------------------------------------
// REDIS KEYS - HASHTAGS & CANONICALISATION
// ---------------------------------------------------------
const (
	//RedisKeyHashtagCanonMap = "most_cache:hashtag:canon:map"
	RedisKeyActiveTagsSet = "most_cache:tags:active:set"
)

// Limites de cache pour les flux "Most" (Top) et les posts d'un utilisateur
const (
	MaxTagElements       = 5000 // On ne garde que les 5000 posts les plus hauts par tag
	MaxRankElements      = 5000 // On ne garde que le top 5000 (Likes, Vues, Global, Recent)
	MaxUserPostsElements = 100  // On ne garde que les 100 derniers posts d'un utilisateur en RAM
)
