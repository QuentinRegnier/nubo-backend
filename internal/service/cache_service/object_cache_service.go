package cache_service

import (
	"context"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/vmihailenco/msgpack/v5"
)

// GetPostsView : Le Pipeline d'Hydratation Optimisé (L1 Redis → L2 Mongo → L3 Postgres)
func GetPostsView(ids []int64) ([]models.PostRequest, error) {
	if len(ids) == 0 {
		return []models.PostRequest{}, nil
	}

	ctx := context.Background()
	finalPosts := make([]models.PostRequest, 0, len(ids))
	tempMap := make(map[int64]models.PostRequest)

	// ========================================================================
	// NIVEAU 1 : REDIS MGET (Ultra Rapide)
	// ========================================================================
	result, err := redis.Posts.GetMany(ctx, ids)
	if err != nil {
		log.Printf("⚠️ Redis MGET error: %v (fallback vers L2)", err)
		result = &redis.GetManyResult{MissingIDs: ids}
	} else {
		for id, data := range result.Found {
			var p models.PostRequest
			// Décodage du binaire MsgPack au lieu du JSON
			if msgpack.Unmarshal(data, &p) == nil {
				tempMap[id] = p
			} else {
				result.MissingIDs = append(result.MissingIDs, id)
			}
		}
	}

	// ========================================================================
	// NIVEAU 2 : MONGO FALLBACK (Pour les trous du Cache RAM)
	// ========================================================================
	var stillMissingIDs []int64

	if len(result.MissingIDs) > 0 {
		mongoPosts, err := mongo.MongoLoadPosts(result.MissingIDs)

		if err == nil {
			mongoFound := make(map[int64]bool)

			// Traitement des posts trouvés
			for _, p := range mongoPosts {
				tempMap[p.ID] = p
				mongoFound[p.ID] = true

				// ⬆️ PROMOTION L2 -> L1 (Réparation du Cache RAM)
				go func(post models.PostRequest) {
					_ = redis.Posts.SetObject(context.Background(), post.ID, post)
				}(p)
			}

			// Identifier ce qui manque ENCORE après l'étape Mongo
			for _, id := range result.MissingIDs {
				if !mongoFound[id] {
					stillMissingIDs = append(stillMissingIDs, id)
				}
			}
		} else {
			log.Printf("⚠️ Mongo Fallback error: %v", err)
			stillMissingIDs = result.MissingIDs // Si Mongo plante, on cherchera tout dans Postgres
		}
	}

	// ========================================================================
	// NIVEAU 3 : POSTGRES FALLBACK (La Source de Vérité Absolue)
	// ========================================================================
	if len(stillMissingIDs) > 0 {
		log.Printf("🛡️ Postgres Fallback déclenché pour %d posts manquants", len(stillMissingIDs))

		// 1. Appel de ta NOUVELLE FONCTION (on met limit = taille du tableau)
		posts, err := postgres.FuncLoadPosts(stillMissingIDs, len(stillMissingIDs), 0)

		if err != nil {
			log.Printf("⚠️ Postgres Fallback error: %v", err)
		} else {
			// 2. Boucle sur les posts propres retournés par la fonction
			for _, p := range posts {
				tempMap[p.ID] = p

				// ⬆️ PROMOTION L3 -> L2 & L1 (Auto-Guérison du Système)
				go func(post models.PostRequest) {
					// 1. Réparer Mongo
					doc, _ := pkg.ToMap(post)
					if doc != nil {
						_ = mongo.Posts.Set(doc) // Assure-toi que cela fait bien un Upsert
					}
					// 2. Réparer Redis
					_ = redis.Posts.SetObject(context.Background(), post.ID, post)
				}(p)
			}
		}
	}

	// ========================================================================
	// ASSEMBLAGE FINAL (Garantit que l'ordre des IDs demandés est respecté)
	// ========================================================================
	for _, id := range ids {
		if p, ok := tempMap[id]; ok {
			finalPosts = append(finalPosts, p)
		}
	}

	return finalPosts, nil
}
