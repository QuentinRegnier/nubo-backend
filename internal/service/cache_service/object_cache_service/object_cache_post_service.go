package object_cache_service

import (
	"context"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	redisgogo "github.com/QuentinRegnier/nubo-backend/internal/pkg/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

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

// IsPostInObjectCache vérifie silencieusement et rapidement si un post est en RAM (O(1))
func IsPostInObjectCache(ctx context.Context, postID int64) bool {
	key := fmt.Sprintf("object:post:%d", postID)
	// Ta fonction Exists renvoie déjà (bool, error), pas besoin de .Result() !
	exists, _ := redisgogo.Exists(ctx, key)
	return exists
}
