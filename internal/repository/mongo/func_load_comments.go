package mongo

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/comment_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"go.mongodb.org/mongo-driver/bson"
)

// MongoLoadComments charge une liste de commentaires depuis MongoDB via leurs IDs (Snowflake)
func MongoLoadComments(c context.Context, commentIDs []int64) ([]comment_models.CommentPayload, error) {
	if len(commentIDs) == 0 {
		return nil, nil
	}

	// Requête stricte avec l'opérateur $in
	filter := bson.M{"id": bson.M{"$in": commentIDs}}

	// On récupère les documents bruts
	docs, err := Comments.GetPaginated(filter, nil, 0, int64(len(commentIDs)))
	if err != nil {
		numan_log.Error(c).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
		return nil, numan_error.NewInternal()
	}

	// Conversion propre dans le type du domaine
	var comments []comment_models.CommentPayload
	for _, doc := range docs {
		var c comment_models.CommentPayload
		if err := pkg.ToStruct(doc, &c); err == nil {
			comments = append(comments, c)
		}
	}

	return comments, nil
}
