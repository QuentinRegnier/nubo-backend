package worker

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// handleGraphUpdate intercepte les créations de posts pour tisser le graphe de tags
func handleGraphUpdate(ctx context.Context, events []redis.AsyncEvent) {
	for _, evt := range events {
		// On s'intéresse uniquement aux publications fraîchement créées
		if evt.Type == redis.EntityPost && evt.Action == redis.ActionCreate {
			jsonBytes, err := json.Marshal(evt.Payload)
			if err != nil {
				continue
			}

			var post post_models.PostPayload
			if err := json.Unmarshal(jsonBytes, &post); err == nil {
				// ✅ Fusionner pour le calcul sémantique
				allTags := append(post.Hashtags, post.IndirectHashtags...)
				if len(allTags) > 1 {
					cache_service.UpdateTagCooccurrences(ctx, allTags, post.CreatedAt)
				}
			}
		}
	}
}
