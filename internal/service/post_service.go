package service

import (
	"context"
	"database/sql"
	"github.com/vmihailenco/msgpack/v5"
	"log"
	"mime/multipart"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/infrastructure/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/lib/pq"
)

// CreatePost (Inchangé)
func CreatePost(userID int64, input domain.CreatePostInput, files []*multipart.FileHeader) (int64, error) {
	now := time.Now().UTC()
	postID := pkg.GenerateID()
	var mediaIDs []int64

	// 1. Upload Images
	for _, fileHeader := range files {
		mediaID := pkg.GenerateID()
		file, err := fileHeader.Open()
		if err == nil {
			go func() {
				_ = UploadMedia(file, "post_media", userID, mediaID)
			}()
			mediaIDs = append(mediaIDs, mediaID)
		}
	}

	// 2. Création Objet
	post := domain.PostRequest{
		ID:          postID,
		UserID:      userID,
		Content:     pkg.CleanStr(input.Content),
		Hashtags:    input.Hashtags,
		Identifiers: input.Identifiers,
		MediaIDs:    mediaIDs,
		Visibility:  input.Visibility,
		Location:    input.Location,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 3. Cache Redis (LFU Init)
	if err := redis.Posts.SetObject(context.Background(), post.ID, post); err != nil {
		return -1, err
	}

	// 4. Persistance Async
	err := redis.EnqueueDB(context.Background(), postID, userID, redis.EntityPost, redis.ActionCreate, post, redis.TargetAll)
	return postID, err
}

// GetPostsView : Le Pipeline d'Hydratation Optimisé (L1 Redis -> L2 Mongo -> L3 Postgres)
func GetPostsView(ids []int64) ([]domain.PostRequest, error) {
	if len(ids) == 0 {
		return []domain.PostRequest{}, nil
	}

	ctx := context.Background()
	finalPosts := make([]domain.PostRequest, 0, len(ids))
	tempMap := make(map[int64]domain.PostRequest)

	// ========================================================================
	// NIVEAU 1 : REDIS MGET (Ultra Rapide)
	// ========================================================================
	result, err := redis.Posts.GetMany(ctx, ids)
	if err != nil {
		log.Printf("⚠️ Redis MGET error: %v (fallback vers L2)", err)
		result = &redis.GetManyResult{MissingIDs: ids}
	} else {
		for id, data := range result.Found {
			var p domain.PostRequest
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
				go func(post domain.PostRequest) {
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

		// Remplacement par la fonction SQL pour appliquer les règles métier (ex: ignorer visibility = 2)
		// ARRAY[0, 1]::smallint[] permet de ne charger que les posts publics (0) ou abonnés (1)
		query := `SELECT id, user_id, content, hashtags, identifiers, media_ids, visibility, location, created_at, updated_at 
				  FROM content.func_load_posts(NULL, $1, ARRAY[0, 1]::smallint[], 0)`

		rows, err := postgres.PostgresDB.QueryContext(ctx, query, pq.Array(stillMissingIDs))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var p domain.PostRequest
				var location sql.NullString // Gestion du NULL en base
				var likeCount int           // Variable poubelle/temporaire car la fonction SQL renvoie le like_count en plus

				err := rows.Scan(
					&p.ID,
					&p.UserID,
					&p.Content,
					pq.Array(&p.Hashtags),
					pq.Array(&p.Identifiers),
					pq.Array(&p.MediaIDs),
					&p.Visibility,
					&location,
					&p.CreatedAt,
					&p.UpdatedAt,
					&likeCount, // On capte la dernière colonne renvoyée par func_load_posts
				)

				if err == nil {
					if location.Valid {
						p.Location = location.String
					}
					tempMap[p.ID] = p

					// ⬆️ PROMOTION L3 -> L2 & L1 (Auto-Guérison du Système)
					go func(post domain.PostRequest) {
						// 1. Réparer Mongo
						doc, _ := pkg.ToMap(post)
						if doc != nil {
							_ = mongo.Posts.Set(doc) // Assure-toi que cela fait bien un Upsert (écrasement si existant)
						}
						// 2. Réparer Redis
						_ = redis.Posts.SetObject(context.Background(), post.ID, post)
					}(p)
				} else {
					log.Printf("⚠️ Erreur Scan Postgres Post %v", err)
				}
			}
		} else {
			log.Printf("⚠️ Postgres Fallback error: %v", err)
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
