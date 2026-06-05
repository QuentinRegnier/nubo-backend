package post_service

import (
	"context"
	"mime/multipart"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// CreatePost (Inchangé)
func CreatePost(userID int64, input post_models.CreatePostInput, files []*multipart.FileHeader) (int64, error) {
	now := time.Now().UTC()
	postID := pkg.GenerateID()
	var mediaIDs []int64

	// 1. Upload Images
	for _, fileHeader := range files {
		mediaID := pkg.GenerateID()
		file, err := fileHeader.Open()
		if err == nil {
			go func() {
				_ = media_service.UploadMedia(file, userID, mediaID)
			}()
			mediaIDs = append(mediaIDs, mediaID)
		}
	}

	// 2. Création Objet
	post := post_models.PostPayload{
		ID:            postID,
		UserID:        userID,
		Content:       pkg.CleanStr(input.Content),
		Hashtags:      input.Hashtags,
		Identifiers:   input.Identifiers,
		MediaIDs:      mediaIDs,
		Visibility:    input.Visibility,
		Location:      input.Location,
		CreatedAt:     now,
		UpdatedAt:     now,
		LikeCount:     0,
		CommentCount:  0,
		ViewCount:     0,
		HasMedia:      len(mediaIDs) > 0,
		VectorVersion: 1, // On initialise la version du vecteur
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2.5 VECTORISATION SYNCHRONE DU CONTENU (O(1) - Très rapide)
	// ─────────────────────────────────────────────────────────────────────────
	// On génère le vecteur mathématique [224]float32 ici, AVANT la persistance.
	// Cela garantit que L1 Cache, MongoDB et PostgreSQL recevront l'objet complet.
	// (Adapte le nom de la fonction selon ton vectorization_service.go)
	post.Vector = feed_service.ComputeContentVectorFull(post, nil)

	// 3. Cache Redis (LFU Init)
	if err := object_cache_service.SetPostInObjectCache(context.Background(), post); err != nil {
		return -1, err
	}

	// ✅ Mise à jour de la timeline de l'utilisateur en temps réel (ZSET L1)
	_ = cache_service.AddPostToUserProfile(context.Background(), userID, postID, float64(now.UnixMilli()))

	// 4. Persistance Async
	// On passe 0 en partitionKey pour que le CRC32 se fasse sur postID.
	// Les futurs Likes utiliseront ce postID pour tomber dans le même Shard !
	err := redis.EnqueueDB(context.Background(), postID, 0, redis.EntityPost, redis.ActionCreate, post, redis.TargetAll)
	return postID, err
}
