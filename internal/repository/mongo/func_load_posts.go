package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoLoadPosts récupère une liste de posts en fonction de leurs IDs (Niveau 2 Fallback)
func MongoLoadPosts(ids []int64) ([]post_models.PostPayload, error) {
	if len(ids) == 0 {
		return []post_models.PostPayload{}, nil
	}

	filter := map[string]any{
		"id": map[string]any{"$in": ids},
	}

	docs, err := Posts.Get(filter, nil)
	if err != nil {
		return nil, err
	}

	var posts []post_models.PostPayload
	for _, doc := range docs {
		var p post_models.PostPayload
		if err := pkg.ToStruct(doc, &p); err == nil {
			posts = append(posts, p)
		}
	}

	return posts, nil
}
