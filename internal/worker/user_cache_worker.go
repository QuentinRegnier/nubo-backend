package worker

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// updateUserCache intercepte les événements pour alimenter les ZSETs spécifiques aux profils utilisateurs (User Cache)
func updateUserCache(ctx context.Context, events []redis.AsyncEvent) {
	for _, e := range events {

		// 1. SI C'EST UN NOUVEAU POST OU UNE MISE À JOUR
		if e.Type == redis.EntityPost && (e.Action == redis.ActionCreate || e.Action == redis.ActionUpdate) {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post post_models.PostPayload
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					// ✅ Ajout du post dans la vitrine du profil de l'utilisateur
					_ = cache_service.AddPostToUserProfile(ctx, post.UserID, post.ID, float64(post.CreatedAt.UnixMilli()))
				}
			}
		}

		// 2. SI C'EST UNE SUPPRESSION DE POST
		if e.Type == redis.EntityPost && e.Action == redis.ActionDelete {
			jsonBytes, err := json.Marshal(e.Payload)
			if err == nil {
				var post post_models.PostPayload
				if err := json.Unmarshal(jsonBytes, &post); err == nil {
					// ✅ Retrait propre du post de la vitrine de l'utilisateur
					_ = cache_service.RemovePostFromUserProfile(ctx, post.UserID, post.ID)
				}
			}
		}
	}
}
