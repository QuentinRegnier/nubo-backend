package cache_service

import (
	"context"
	"log"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/vmihailenco/msgpack/v5"
)

// GetPostsView : Le Pipeline d'Hydratation Optimisé (L1 Redis → L2 Mongo → L3 Postgres)
func GetPostsView(ids []int64) ([]post_models.PostPayload, error) {
	if len(ids) == 0 {
		return []post_models.PostPayload{}, nil
	}

	ctx := context.Background()
	finalPosts := make([]post_models.PostPayload, 0, len(ids))
	tempMap := make(map[int64]post_models.PostPayload)

	// ========================================================================
	// NIVEAU 1 : REDIS MGET (Ultra Rapide)
	// ========================================================================
	result, err := redis.Posts.GetMany(ctx, ids)
	if err != nil {
		log.Printf("⚠️ Redis MGET error: %v (fallback vers L2)", err)
		result = &redis.GetManyResult{MissingIDs: ids}
	} else {
		for id, data := range result.Found {
			var p post_models.PostPayload
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
				go func(post post_models.PostPayload) {
					_ = SetPostInObjectCache(context.Background(), post)
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
				go func(post post_models.PostPayload) {
					// 1. Réparer Mongo
					doc, _ := pkg.ToMap(post)
					if doc != nil {
						_ = mongo.Posts.Set(doc) // Assure-toi que cela fait bien un Upsert
					}
					// 2. Réparer Redis
					_ = SetPostInObjectCache(context.Background(), post)
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

// --- GESTION DES POSTS (CACHE L1) ---

// GetPostFromObjectCache récupère un post depuis le cache L1
func GetPostFromObjectCache(ctx context.Context, postID int64) (post_models.PostPayload, error) {
	var p post_models.PostPayload
	err := redis.Posts.GetObject(ctx, postID, &p)
	return p, err
}

// SetPostInObjectCache enregistre ou met à jour un post dans le cache L1
func SetPostInObjectCache(ctx context.Context, post post_models.PostPayload) error {
	return redis.Posts.SetObject(ctx, post.ID, post)
}

// DeletePostFromObjectCache purge instantanément un post du cache L1
func DeletePostFromObjectCache(ctx context.Context, postID int64) error {
	// Utilisation propre de la méthode native du wrapper Redis
	return redis.Posts.DeleteObject(ctx, postID)
}

// --- GESTION DES MÉDIAS (CACHE L1) ---

// SetMediaInObjectCache place les métadonnées de l'image en RAM (Write-Behind)
func SetMediaInObjectCache(ctx context.Context, media models.MediaRequest) error {
	// Le TTL (ex: 24h) est défini dans le manager Redis et se réinitialise à chaque GET
	return redis.Media.SetObject(ctx, media.ID, media)
}

// GetMediaFromObjectCache récupère instantanément les métadonnées (O(1))
func GetMediaFromObjectCache(ctx context.Context, mediaID int64) (models.MediaRequest, error) {
	var m models.MediaRequest
	err := redis.Media.GetObject(ctx, mediaID, &m)
	return m, err
}

// DeleteMediaFromObjectCache purge les métadonnées de la RAM
func DeleteMediaFromObjectCache(ctx context.Context, mediaID int64) error {
	return redis.Media.DeleteObject(ctx, mediaID)
}
