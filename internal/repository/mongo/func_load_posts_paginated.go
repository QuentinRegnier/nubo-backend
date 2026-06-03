package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoLoadPostsPaginated récupère des posts avec filtres, tri et pagination (Cold Storage)
func MongoLoadPostsPaginated(filter map[string]any, sort map[string]any, skip int64, limit int64) ([]post_models.PostPayload, error) {
	docs, err := Posts.GetPaginated(filter, sort, skip, limit)
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
