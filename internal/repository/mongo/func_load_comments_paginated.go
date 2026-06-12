package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"go.mongodb.org/mongo-driver/bson"
)

// MongoLoadCommentsPaginated lit L2 en utilisant un Index Composé B-Tree ultra-rapide
func MongoLoadCommentsPaginated(postID int64, offset int64, limit int64) ([]comment_models.CommentPayload, error) {
	// Filtre de base
	filter := bson.M{
		"post_id":    postID,
		"visibility": bson.M{"$ne": -1}, // Ignore les Soft Deletes
	}

	// Tri compatible avec ton wrapper (map[string]any)
	// -1 pour DESC (plus hauts scores en premier), 1 pour ASC (plus anciens en premier)
	sortMap := map[string]any{
		"score":      -1,
		"created_at": 1,
	}

	// Appel propre de ton wrapper métier
	docs, err := Comments.GetPaginated(filter, sortMap, offset, limit)
	if err != nil {
		return nil, err
	}

	var comments []comment_models.CommentPayload
	for _, doc := range docs {
		var c comment_models.CommentPayload
		if err := pkg.ToStruct(doc, &c); err == nil {
			comments = append(comments, c)
		}
	}
	return comments, nil
}
