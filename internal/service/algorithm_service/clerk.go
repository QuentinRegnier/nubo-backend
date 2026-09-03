package algorithm_service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// CollectCandidates construit les 3 paniers (A, B, C) avec leurs ADN respectifs
func CollectCandidates(ctx context.Context, userID int64, seeds [3]int64, quotas Quotas) (*FeedBaskets, error) {
	if err := quotas.Validate(); err != nil {
		return nil, err // L'erreur est déjà une AppError propre !
	}

	baskets := NewFeedBaskets(quotas.MaxCandidates, seeds[0], seeds[1], seeds[2])

	// ─────────────────────────────────────────────────────────────────────────────
	// ACTION 1 : Le Socle Social (Boîte aux lettres)
	// ─────────────────────────────────────────────────────────────────────────────
	_ = baskets.LoadSocialMailbox(ctx, userID)

	// ─────────────────────────────────────────────────────────────────────────────
	// ACTION 2 : Fusion Télémétrie / Graph 1-Hop / Leaderboard Mondial
	// ─────────────────────────────────────────────────────────────────────────────
	tagCloud := buildTagCloud(ctx, userID) // ✅ NOUVEAU : Transmission du userID

	// ─────────────────────────────────────────────────────────────────────────────
	// REMPLISSAGE DÉTERMINISTE DES 3 PANIERS
	// ─────────────────────────────────────────────────────────────────────────────
	fillBasket(ctx, userID, baskets.A, quotas, tagCloud)
	fillBasket(ctx, userID, baskets.B, quotas, tagCloud)
	fillBasket(ctx, userID, baskets.C, quotas, tagCloud)

	return baskets, nil
}

// CollectSingleBasket construit un unique panier avec son ADN strict (Cas 3 : Extension)
// Utilise la Seed du flux actif pour garantir la continuité de l'identité algorithmique.
func CollectSingleBasket(ctx context.Context, userID int64, seed int64, quotas Quotas) (*CandidateBasket, error) {
	if err := quotas.Validate(); err != nil {
		return nil, err
	}

	// Astuce : On utilise la mécanique FeedBaskets pour charger la boîte aux lettres,
	// mais on ne garde et ne remplit que le panier A.
	baskets := NewFeedBaskets(quotas.MaxCandidates, seed, seed, seed)
	_ = baskets.LoadSocialMailbox(ctx, userID)

	singleBasket := baskets.A

	// Fusion Télémétrie / Graph 1-Hop / Leaderboard
	tagCloud := buildTagCloud(ctx, userID) // ✅ NOUVEAU : Transmission du userID

	// Remplissage ciblé
	fillBasket(ctx, userID, singleBasket, quotas, tagCloud)

	return singleBasket, nil
}

// buildTagCloud abstrait la création du Super-Nuage sémantique pour éviter la duplication de code.
// ✅ NOUVEAU : Elle accepte userID pour pouvoir interroger la télémétrie de cet utilisateur précis.
func buildTagCloud(ctx context.Context, userID int64) map[string]float64 {

	// ─────────────────────────────────────────────────────────────────────────────
	// 1. LECTURE DE LA TÉLÉMÉTRIE (L1 SPEED CACHE)
	// ─────────────────────────────────────────────────────────────────────────────
	userTelemetry := make(map[string]float64)

	// On récupère le "TopTags" que le téléphone a envoyé lors du dernier /sync/telemetry.
	// La méthode GetTelemetryTags est ultra-rapide (O(1)) car elle tape en RAM.
	topTags, err := cache_service.GetTelemetryTags(ctx, userID)

	if err == nil && len(topTags) > 0 {
		// La télémétrie renvoie un tableau classé du plus fort au plus faible.
		// On va leur attribuer un poids décroissant.
		// Ex: Le 1er tag vaut 1.0, le 2ème vaut 0.9, le 3ème vaut 0.8...
		weight := 1.0
		for _, tag := range topTags {
			userTelemetry[tag] = weight
			weight -= 0.1 // On baisse le poids de 10% pour le tag suivant
			if weight < 0.1 {
				weight = 0.1 // Poids minimum
			}
		}
	} else {
		// 🚨 FALLBACK UX : Cold Start (Nouvel utilisateur ou Télémétrie absente)
		// On utilise les tags du Leaderboard mondial (les sujets les plus chauds du moment)
		// pour amorcer la pompe et lui proposer du contenu qualitatif par défaut.
		leaderboardData, errL := redis.ZRevRangeWithScores(ctx, variables.RedisKeyHashtagLeaderboard, 0, 4)
		if errL == nil && len(leaderboardData) > 0 {
			weight := 1.0
			for _, z := range leaderboardData {
				userTelemetry[z.Member.(string)] = weight
				weight -= 0.1
			}
		} else {
			// Dernier filet de sécurité (Si même le Leaderboard est vide, ex: Reset BDD)
			userTelemetry["bienvenue"] = 1.0
		}
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// 2. LECTURE DU LEADERBOARD (Pour les Multiplicateurs de Viralité)
	// ─────────────────────────────────────────────────────────────────────────────
	leaderboardData, _ := redis.ZRevRangeWithScores(ctx, variables.RedisKeyHashtagLeaderboard, 0, 49)
	leaderboardBoosts := make(map[string]float64)
	if len(leaderboardData) > 0 {
		maxScore := leaderboardData[0].Score
		for _, z := range leaderboardData {
			// Normalisation du boost (Le #1 mondial donnera un boost de 1.5x)
			if maxScore > 0 {
				leaderboardBoosts[z.Member.(string)] = 1.0 + (z.Score / maxScore * 0.5)
			}
		}
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// 3. EXPANSION ET ASSEMBLAGE DU SUPER-NUAGE (L'Effet Pingouin)
	// ─────────────────────────────────────────────────────────────────────────────
	tagCloud := make(map[string]float64)
	for coreTag, affinity := range userTelemetry {
		boost := 1.0
		if val, ok := leaderboardBoosts[coreTag]; ok {
			boost = val
		}
		tagCloud[coreTag] += affinity * 1.0 * boost

		// L'EFFET PINGOUIN (Expansion 1-Hop Mathématique)
		neighbors := cache_service.GetRelatedTagsLazy(ctx, coreTag)
		for neighbor, edgeWeight := range neighbors {
			nBoost := 1.0
			if val, ok := leaderboardBoosts[neighbor]; ok {
				nBoost = val
			}
			tagCloud[neighbor] += affinity * edgeWeight * nBoost
		}
	}

	return tagCloud
}

// fillBasket remplit un panier spécifique en respectant les quotas et en appliquant l'expansion dynamique.
func fillBasket(ctx context.Context, userID int64, basket *CandidateBasket, quotas Quotas, initialTagCloud map[string]float64) {
	globalTarget := int(float64(quotas.MaxCandidates) * quotas.GlobalRatio)
	tagTarget := int(float64(quotas.MaxCandidates) * quotas.TagRatio)

	// ─────────────────────────────────────────────────────────────────────────────
	// 1. COLLECTE GLOBALE (Avec report du déficit)
	// ─────────────────────────────────────────────────────────────────────────────
	dateKey := time.Now().UTC().Format("20060102")
	globalKey := fmt.Sprintf(variables.RedisKeyTrendGlobalDaily, dateKey)

	globalAdded := basket.FetchDeterministicallyFromZSET(ctx, userID, globalKey, globalTarget, OriginGlobal)

	// ✅ S'il manque des posts globaux (ZSET épuisé ou doublons), on reporte la charge sur les tags
	if globalAdded < globalTarget {
		deficit := globalTarget - globalAdded
		tagTarget += deficit
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// 2. COLLECTE CIBLÉE (Nuage de Tags Dynamique)
	// ─────────────────────────────────────────────────────────────────────────────
	tagCloud := make(map[string]float64)
	for k, v := range initialTagCloud {
		tagCloud[k] = v // Clone pour ne pas altérer la base commune aux autres paniers
	}

	tagAdded := 0
	depth := 1
	maxDepth := 2 // ✅ 1 = Nuage initial, 2 = 1-Hop (Cousins directs).

	// Boucle dynamique : on itère tant qu'il manque des posts et qu'on n'a pas atteint le fond du graphe
	for tagAdded < tagTarget && depth <= maxDepth {
		totalWeight := 0.0
		for _, weight := range tagCloud {
			totalWeight += weight
		}

		addedInThisDepth := 0

		// Tirage dans le nuage actuel
		for tag, weight := range tagCloud {
			if tagAdded >= tagTarget {
				break
			}

			// Demande proportionnelle au poids du tag dans le nuage
			targetForTag := int(math.Ceil(float64(tagTarget-tagAdded) * (weight / totalWeight)))
			if targetForTag <= 0 {
				targetForTag = 1
			}

			// Note : Assure-toi que la variable correspond à ton nommage Redis exact
			key := fmt.Sprintf("trend:tag:%s:daily", tag)
			added := basket.FetchDeterministicallyFromZSET(ctx, userID, key, targetForTag, OriginTag)

			addedInThisDepth += added
			tagAdded += added
		}

		if tagAdded >= tagTarget {
			break // Objectif final atteint
		}

		// ✅ EXPANSION DYNAMIQUE (Le quota n'est pas rempli, on creuse le graphe sémantique)
		newTags := make(map[string]float64)
		for tag, weight := range tagCloud {
			neighbors := cache_service.GetRelatedTagsLazy(ctx, tag) // Recherche des voisins
			for neighbor, edgeWeight := range neighbors {
				// Si c'est un tout nouveau tag, on l'ajoute avec un poids atténué par la profondeur
				if _, exists := tagCloud[neighbor]; !exists {
					newTags[neighbor] = weight * edgeWeight * 0.8
				}
			}
		}

		// 🛑 UX LIMIT : On bloque la dérive sémantique stricte au 1-Hop.
		if len(newTags) == 0 || depth >= maxDepth {
			break
		}

		// On fusionne les nouveaux tags découverts pour la prochaine itération (Plongée +1)
		for k, v := range newTags {
			tagCloud[k] = v
		}
		depth++
	}

	// ✅ FALLBACK UX (L'Éléphant partiel) : Si la niche est épuisée, on remplit le déficit
	// avec du contenu Viral Mondial (Bangers) plutôt que de proposer du hors-sujet.
	if tagAdded < tagTarget {
		deficit := tagTarget - tagAdded
		dateKey := time.Now().UTC().Format("20060102")
		globalKey := fmt.Sprintf(variables.RedisKeyTrendGlobalDaily, dateKey)
		basket.FetchDeterministicallyFromZSET(ctx, userID, globalKey, deficit, OriginGlobal)
	}
}
