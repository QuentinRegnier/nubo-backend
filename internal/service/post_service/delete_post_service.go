package post_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
)

// DeletePost gère la rétractation d'un post (Purge L1, Purge LSH, Soft Delete Workers).
func DeletePost(ctx context.Context, input post_models.DeletePostInput) error {
	var post models.PostRequest
	var found bool

	// ─────────────────────────────────────────────────────────────────────────
	// 1. LECTURE CASCADE (On a besoin de l'objet pour les hashtags et la sécu)
	// ─────────────────────────────────────────────────────────────────────────
	if err := redis.Posts.GetObject(ctx, input.PostID, &post); err == nil {
		found = true
	} else {
		mongoPosts, errMongo := mongo.MongoLoadPosts([]int64{input.PostID})
		if errMongo == nil && len(mongoPosts) > 0 {
			post = mongoPosts[0]
			found = true
		} else {
			pgPosts, errPg := postgres.FuncLoadPosts([]int64{input.PostID}, 1, 0)
			if errPg == nil && len(pgPosts) > 0 {
				post = pgPosts[0]
				found = true
			}
		}
	}

	if !found {
		return errors.New("not found")
	}

	// 🛡 SÉCURITÉ : Vérification du propriétaire
	if post.UserID != input.UserID {
		return errors.New("unauthorized")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. PURGE SYNCHRONE DES CACHES (Disparition instantanée pour les utilisateurs)
	// ─────────────────────────────────────────────────────────────────────────

	// A. Suppression de l'Object Cache L1
	keyL1 := fmt.Sprintf("object:post:%d", input.PostID)
	_ = redisgo.Rdb.Del(ctx, keyL1).Err() // Ignore l'erreur si la clé a déjà expiré

	// B. Suppression du seau LSH et du Vecteur
	// Il faut d'abord lire le vecteur pour récupérer le LSHHash, puis nettoyer le bucket
	vecKey := fmt.Sprintf("content:vec:%d", input.PostID)
	if vecData, err := redisgo.Rdb.Get(ctx, vecKey).Bytes(); err == nil {
		var payload feed_service.ContentVectorPayload
		if err := json.Unmarshal(vecData, &payload); err == nil {
			// On a le hash, on peut retirer le post de son bucket LSH
			_ = feed_service.RemoveLSHBucket(ctx, input.PostID, payload.LSHHash)
		}
	}
	// Purge définitive du vecteur d'engagement du post
	_ = redisgo.Rdb.Del(ctx, vecKey).Err()

	// ─────────────────────────────────────────────────────────────────────────
	// 3. ENVOI AUX WORKERS POUR SOFT-DELETE (BDD et MOST Cache)
	// ─────────────────────────────────────────────────────────────────────────

	// On passe l'objet `post` ENTIER dans le payload. C'est crucial pour que
	// `most_cache_worker.go` puisse lire `post.Hashtags` et nettoyer les bons ZSETs.
	return redis.EnqueueDB(ctx, post.ID, 0, redis.EntityPost, redis.ActionDelete, post, redis.TargetAll)
}
