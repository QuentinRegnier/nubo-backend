package cache

import (
	"context"
	"fmt"
	"log"
	"strconv"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// 4. AMORÇAGE (SEEDING)
// ============================================================================

// SeedMostCache lit l'intégralité de Postgres pour populer le L1 (RAM), L2 (Mongo) et le MOST Cache.
func SeedMostCache() error {
	ctx := context.Background()

	// ---------------------------------------------------------
	// PHASE 1 : RESTAURATION DU SYSTÈME DE TAGS
	// ---------------------------------------------------------
	log.Println("♻️ Restauration des tags communautaires depuis SQL...")

	// Appel unique et propre !
	tagsToSync, err := postgres.FuncLoadAllTags()
	if err != nil {
		log.Printf("⚠️ Erreur lors du chargement des tags : %v", err)
	} else if len(tagsToSync) > 0 {
		// On utilise SAdd pour restaurer le SET Redis (Source pour le Cron Canoniseur)
		// On convertit en []interface{} pour le driver Redis
		args := make([]interface{}, len(tagsToSync))
		for i, v := range tagsToSync {
			args[i] = v
		}
		_ = redisgo.Rdb.SAdd(ctx, variables.RedisKeyActiveTagsSet, args...).Err()
	}

	// ---------------------------------------------------------
	// PHASE 2 : HYDRATATION DES POSTS ET CLASSEMENTS (PAR BLOCS)
	// ---------------------------------------------------------
	log.Println("♻️ Hydratation du MOST Cache depuis SQL (Mode Paginated)...")

	limit := 10000 // Blocs de 10 000 posts pour préserver la RAM
	offset := 0
	totalProcessed := 0

	for {
		posts, err := postgres.FuncLoadPostsPaginated(limit, offset)
		if err != nil {
			return fmt.Errorf("erreur requête seeding (offset %d): %w", offset, err)
		}

		if len(posts) == 0 {
			break // La base entière a été scannée
		}

		for _, p := range posts {
			// ⚠️ SUPPRESSION DE L'HYDRATATION L1/L2 ICI !
			// On ne charge surtout pas les millions de posts en RAM ou dans Mongo pendant le scan.
			// On laisse le script Lua faire son travail d'élimination impitoyable.

			// 1. Écrémage Mathématique : Routage Temporel et par Tags (TDD)
			// La fonction calcule S(p, t) avec la pénalité de temps actuel et appelle ZAddWithCap.
			// Le script Lua va insérer l'ID, vérifier si le bucket dépasse 500, et éjecter le pire instantanément.
			UpdatePostRecommendationScore(ctx, p)

			// 2. Classements STRICTS (Interface Utilisateur)
			// Même logique atomique, mais plafonnée à MaxStrictElements (5000).
			_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictLikes, float64(p.LikeCount), p.ID, variables.MaxStrictElements)
			_ = redis.ZAddWithCap(ctx, variables.RedisKeyStrictViews, float64(p.ViewCount), p.ID, variables.MaxStrictElements)
		}

		totalProcessed += len(posts)
		log.Printf("⏳ Seeding en cours... %d posts traités.", totalProcessed)
		offset += limit
	}

	// ---------------------------------------------------------
	// PHASE 3 : HYDRATATION INVERSÉE (PRE-WARMING FINAL)
	// ---------------------------------------------------------
	log.Println("🔥 Lancement de l'hydratation inversée (Pre-warming L1/L2)...")

	// 1. Collecte des IDs gagnants dans tous les rayons trend:*
	winnerIDsMap := make(map[int64]bool)

	// On scanne les clés correspondant à la nomenclature unifiée
	keys, _ := redisgo.Rdb.Keys(ctx, "most_cache:trend:*").Result()
	for _, key := range keys {
		// On récupère tous les membres du ZSET (les IDs des posts d'élite)
		ids, _ := redisgo.Rdb.ZRange(ctx, key, 0, -1).Result()
		for _, idStr := range ids {
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				winnerIDsMap[id] = true
			}
		}
	}

	// 2. Requête massive optimisée vers PostgreSQL pour les gagnants
	if len(winnerIDsMap) > 0 {
		var ids []int64
		for id := range winnerIDsMap {
			ids = append(ids, id)
		}

		winners, err := postgres.FuncLoadPosts(ids, 1, 0) // p_order_mode 0 = récents
		if err == nil {
			for _, p := range winners {
				// Sanctuarisation L1 (Redis Object Cache) - Pas de TTL (0)
				_ = redis.Posts.SetObject(ctx, p.ID, p)

				// Synchronisation L2 (MongoDB)
				doc, _ := pkg.ToMap(p)
				if doc != nil {
					_ = mongo.Posts.Set(doc)
				}
			}
			log.Printf("💎 %d posts d'élite sanctuarisés dans le cache L1.", len(winners))
		}
	}

	// 3. Synchronisation MongoDB (L2) pour le dernier mois
	log.Println("📦 Synchronisation MongoDB pour les posts des 30 derniers jours...")
	recentPosts, err := postgres.FuncLoadRecentPosts(30)
	if err == nil {
		for _, p := range recentPosts {
			doc, _ := pkg.ToMap(p)
			if doc != nil {
				_ = mongo.Posts.Set(doc)
			}
		}
		log.Printf("✅ %d posts récents synchronisés dans MongoDB.", len(recentPosts))
	}

	// 4. Désactivation du flag de maintenance
	// On utilise une clé Redis dédiée pour que toutes les instances de l'API s'ouvrent en même temps
	_ = redisgo.Rdb.Set(ctx, "system:status:maintenance", "off", 0).Err()
	log.Println("🚀 Mode maintenance désactivé. L'API est opérationnelle.")

	return nil
}
