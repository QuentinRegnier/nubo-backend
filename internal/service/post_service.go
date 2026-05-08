package service

import (
	"context"
	"mime/multipart"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
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
				_ = UploadMedia(file, userID, mediaID)
			}()
			mediaIDs = append(mediaIDs, mediaID)
		}
	}

	// 2. Création Objet
	post := domain.PostRequest{
		ID:           postID,
		UserID:       userID,
		Content:      pkg.CleanStr(input.Content),
		Hashtags:     input.Hashtags,
		Identifiers:  input.Identifiers,
		MediaIDs:     mediaIDs,
		Visibility:   input.Visibility,
		Location:     input.Location,
		CreatedAt:    now,
		UpdatedAt:    now,
		LikeCount:    0,
		CommentCount: 0,
		ViewCount:    0,
		HasMedia:     len(mediaIDs) > 0,
	}

	// 3. Cache Redis (LFU Init)
	if err := redis.Posts.SetObject(context.Background(), post.ID, post); err != nil {
		return -1, err
	}

	// 4. Persistance Async
	// On passe 0 en partitionKey pour que le CRC32 se fasse sur postID.
	// Les futurs Likes utiliseront ce postID pour tomber dans le même Shard !
	err := redis.EnqueueDB(context.Background(), postID, 0, redis.EntityPost, redis.ActionCreate, post, redis.TargetAll)
	return postID, err
}
