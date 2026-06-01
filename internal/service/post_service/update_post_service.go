package post_service

import (
	"context"
	"errors"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/feed_service"
)

// UpdatePost gère la modification en récupérant l'objet complet pour nourrir le Bulk Update des workers.
func UpdatePost(ctx context.Context, input post_models.UpdatePostInput) error {
	var post models.PostRequest
	var found bool

	// ─────────────────────────────────────────────────────────────────────────
	// 1. CASCADE DE LECTURE (L1 -> L2 -> L3) POUR HYDRATER L'OBJET COMPLET
	// ─────────────────────────────────────────────────────────────────────────

	// L1 : Object Cache LFU (Redis)
	if err := redis.Posts.GetObject(ctx, input.PostID, &post); err == nil {
		found = true
	} else {
		// L2 : Cold Storage (MongoDB) en utilisant ta fonction existante
		mongoPosts, errMongo := mongo.MongoLoadPosts([]int64{input.PostID})
		if errMongo == nil && len(mongoPosts) > 0 {
			post = mongoPosts[0]
			found = true
		} else {
			// L3 : Source of Truth (PostgreSQL) en utilisant ta fonction existante (à adapter selon le nom exact)
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

	// ─────────────────────────────────────────────────────────────────────────
	// 2. CONTRÔLE D'AUTORISATION
	// ─────────────────────────────────────────────────────────────────────────
	if post.UserID != input.UserID {
		return errors.New("unauthorized")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 3. APPLICATION DES MODIFICATIONS
	// ─────────────────────────────────────────────────────────────────────────
	post.Content = input.Content
	post.Hashtags = input.Hashtags
	post.Identifiers = input.Identifiers
	post.Location = input.Location
	post.Visibility = input.Visibility
	post.UpdatedAt = time.Now().UTC()

	// L'IA locale (Edge Computing) saura qu'il faut recalculer ses affinités
	post.VectorVersion += 1

	// ─────────────────────────────────────────────────────────────────────────
	// 3.5 RE-VECTORISATION SYNCHRONE DU CONTENU
	// ─────────────────────────────────────────────────────────────────────────
	// On met à jour le vecteur avec les nouvelles données textuelles/thématiques.
	post.Vector = feed_service.ComputeContentVectorFull(post, nil)

	// ─────────────────────────────────────────────────────────────────────────
	// 4. SAUVEGARDE ET DÉLÉGATION AUX WORKERS BATCH
	// ─────────────────────────────────────────────────────────────────────────

	// 1. Écrasement LFU immédiat
	if err := redis.Posts.SetObject(ctx, post.ID, post); err != nil {
		return err
	}

	// 2. Envoi de l'objet COMPLET dans la file asynchrone pour que tes bulkUpdate fonctionnent
	return redis.EnqueueDB(ctx, post.ID, 0, redis.EntityPost, redis.ActionUpdate, post, redis.TargetAll)
}
