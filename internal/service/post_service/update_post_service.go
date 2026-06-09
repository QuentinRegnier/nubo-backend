package post_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/algorithm_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/security_service"
)

// UpdatePost gère la modification en récupérant l'objet complet pour nourrir le Bulk Update des workers.
func UpdatePost(ctx context.Context, input post_models.UpdatePostInput) error {
	// ─────────────────────────────────────────────────────────────────────────
	// 1. VERIFICATION DROIT D'ACCÈS ET RÉCUPÉRATION DE L'OBJET COMPLET (Nécessaire pour les étapes suivantes)
	// ─────────────────────────────────────────────────────────────────────────

	post, err := security_service.LeftPost(ctx, input.PostID, input.UserID)
	if err != nil {
		return err
	}

	// ─────────────────────────────────────────────────────────────────────────
	// 2. APPLICATION DES MODIFICATIONS
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
	// 3. RE-VECTORISATION SYNCHRONE DU CONTENU
	// ─────────────────────────────────────────────────────────────────────────
	// On met à jour le vecteur avec les nouvelles données textuelles/thématiques.
	post.Vector = algorithm_service.ComputeContentVectorFull(post, nil)

	// ─────────────────────────────────────────────────────────────────────────
	// 4. SAUVEGARDE ET DÉLÉGATION AUX WORKERS BATCH
	// ─────────────────────────────────────────────────────────────────────────

	// 1. Écrasement LFU immédiat
	if err := object_cache_service.SetPostInObjectCache(ctx, post); err != nil {
		return err
	}

	// 2. Envoi de l'objet COMPLET dans la file asynchrone pour que tes bulkUpdate fonctionnent
	return redis.EnqueueDB(ctx, post.ID, 0, redis.EntityPost, redis.ActionUpdate, post, redis.TargetAll)
}
