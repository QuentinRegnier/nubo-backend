package service

import (
	"context"
	"log"
	"mime/multipart"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

func CreatePost(userID int64, input domain.CreatePostInput, files []*multipart.FileHeader) (int64, error) {
	now := time.Now().UTC()
	postID := pkg.GenerateID()
	var mediaIDs []int64

	// 1. Traitement des images via le MediaService existant
	for _, fileHeader := range files {
		mediaID := pkg.GenerateID()
		file, err := fileHeader.Open()
		if err == nil {
			// Utilisation de votre fonction UploadMedia existante
			go func() {
				err := UploadMedia(file, "post_media", userID, mediaID)
				if err != nil {
					log.Printf("internal error (image upload): %v", err)
					return
				}
			}()
			mediaIDs = append(mediaIDs, mediaID)
		}
	}

	// 2. Préparation de l'objet Post
	post := domain.PostRequest{}
	post.ID = postID
	post.UserID = userID
	post.Content = pkg.CleanStr(input.Content)
	post.Hashtags = input.Hashtags
	post.Identifiers = input.Identifiers
	post.MediaIDs = mediaIDs
	post.Visibility = input.Visibility
	post.Location = input.Location
	post.CreatedAt = now
	post.UpdatedAt = now

	// 3. Cache Redis Immédiat
	if err := redis.RedisCreatePost(post); err != nil {
		return -1, err
	}

	// 4. Persistance Asynchrone (Postgres + Mongo) via la Queue
	ctx := context.Background()
	err := redis.EnqueueDB(
		ctx,
		postID,
		userID, // PartitionKey = userID pour que les posts d'un user soient sur le même shard
		redis.EntityPost,
		redis.ActionCreate,
		post,
		redis.TargetAll,
	)

	return postID, err
}
