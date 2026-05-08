package service

import (
	"context"
	"fmt"
	"log"

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
	// PHASE 2 : HYDRATATION DES POSTS ET CLASSEMENTS
	// ---------------------------------------------------------
	log.Println("♻️ Hydratation du MOST Cache depuis les colonnes matérialisées SQL...")

	// On passe 'nil' pour ne pas filtrer par IDs, et une limite très haute pour le seeding
	posts, err := postgres.FuncLoadPosts(nil, 1000000, 0)
	if err != nil {
		return fmt.Errorf("erreur requête seeding: %w", err)
	}

	for _, p := range posts {
		// A. Hydratation L2 (MongoDB)
		doc, _ := pkg.ToMap(p)
		if doc != nil {
			_ = mongo.Posts.Set(doc)
		}

		// B. Hydratation L1 (OBJECT Cache)
		_ = redis.Posts.SetObject(ctx, p.ID, p)

		// C. Hydratation MOST Cache (ZSETs algorithmiques)
		UpdatePostRecommendationScore(ctx, p.ID, p.Hashtags)

		// D. Classement STRICT (Absolu)
		// On utilise p.LikeCount directement casté en float64
		_ = redis.ZAddWithCap(ctx, variables.RedisKeyRankLikesStrict, float64(p.LikeCount), p.ID, variables.MaxRankElements)

		// E. Hydratation USER Cache (Profil utilisateur)
		// Reconstruction de la grille chronologique avec plafonnement automatique
		AddPostToUserProfile(ctx, p.UserID, p.ID, p.CreatedAt.UnixMilli())
	}

	log.Println("✅ Seeding terminé : synchronisation complète L1, L2, USER Cache et MOST Cache.")
	return nil
}
