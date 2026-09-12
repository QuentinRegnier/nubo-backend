package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"go.mongodb.org/mongo-driver/bson"
)

// MongoGetPinnedIndices récupère tous les indices d'épingles utilisés par l'utilisateur.
func MongoGetPinnedIndices(userID int64) ([]int, error) {
	filter := bson.M{
		"user_id":         userID,
		"settings.pinned": bson.M{"$gte": 0},
	}
	docs, err := Members.GetPaginated(filter, nil, 0, 3)
	if err != nil {
		return nil, err
	}

	var indices []int
	for _, doc := range docs {
		var mem conversation_models.MemberPayload
		if err := pkg.ToStruct(doc, &mem); err == nil && mem.Settings.Pinned >= 0 {
			indices = append(indices, mem.Settings.Pinned)
		}
	}
	return indices, nil
}
