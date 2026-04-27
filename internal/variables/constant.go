package variables

import "time"

const (
	ToleranceTimeSeconds         = 300     // 5 minutes
	JWTExpirationSeconds         = 900     // 15 minutes
	MasterTokenExpirationSeconds = 2592000 // 1 mois en secondes
)

// Configuration des bits pour l'algorithme Snowflake
const (
	Epoch    = int64(1704067200000) // Date de départ : 1er Janvier 2024 (Custom Epoch)
	NodeBits = uint(10)             // 10 bits pour l'ID du noeud (1024 noeuds max)
	StepBits = uint(12)             // 12 bits pour la séquence (4096 IDs par ms)

	NodeMax   = int64(-1 ^ (-1 << NodeBits)) // Max Node ID (1023)
	StepMax   = int64(-1 ^ (-1 << StepBits)) // Max Sequence (4095)
	TimeShift = NodeBits + StepBits          // Décalage pour le timestamp (22)
	NodeShift = StepBits                     // Décalage pour le noeud (12)
)

const MaxTags = 10

// ---------------------------------------------------------
// REDIS KEYS - RECOMMANDATION & TENDANCES (ZSETS)
// ---------------------------------------------------------
const (
	// Mode Strict (Valeurs absolues pures)
	RedisKeyRankLikesStrict  = "rank:likes:strict"
	RedisKeyRankViewsStrict  = "rank:views:strict"
	RedisKeyRankRecentStrict = "rank:recent:strict"

	// Mode Global (Algorithme de Time-Decay avec ou sans boosts)
	RedisKeyRankGlobal         = "rank:global"
	RedisKeyRankLikesGlobal    = "rank:likes:global"
	RedisKeyRankCommentsGlobal = "rank:comments:global"
	RedisKeyRankRecentGlobal   = "rank:recent:global"

	// Tags & Perso
	RedisKeyRankTag       = "idx:tag:%s"     // %s = slug du tag canonique
	RedisKeyContentVector = "content:vec:%d" // %d = post_id
)

// ---------------------------------------------------------
// RECOMMANDATION - MULTIPLICATEURS (BOOSTS)
// ---------------------------------------------------------
const (
	BoostRecent   = 1.8 // Réduit la gravité temporelle pour favoriser la fraîcheur
	BoostLikes    = 1.5 // Augmente le poids des likes de 50%
	BoostComments = 1.5 // Augmente le poids des commentaires de 50%
)

// ---------------------------------------------------------
// REDIS KEYS - HASHTAGS & CANONICALISATION
// ---------------------------------------------------------
const (
	RedisKeyHashtagCanonMap = "hashtag:canon:map" // HASH: Mot-clé (faute) -> Slug officiel
	RedisKeyActiveTagsSet   = "tags:active:set"   // SET: Liste de tous les tags communautaires
)

// Configuration temporelle OBJECT Cache
const (
	StandardTTL = 7 * 24 * time.Hour // TTL STANDARD : 7 Jours (Pragmatique, évite la saturation).
)

// Limites de cache pour les flux "Most" (Top) et les posts d'un utilisateur
const (
	MaxTagElements       = 5000 // On ne garde que les 5000 posts les plus hauts par tag
	MaxRankElements      = 5000 // On ne garde que le top 5000 (Likes, Vues, Global, Recent)
	MaxUserPostsElements = 100  // On ne garde que les 100 derniers posts d'un utilisateur en RAM
)
