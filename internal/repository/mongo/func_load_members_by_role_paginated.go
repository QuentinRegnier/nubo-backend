package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"go.mongodb.org/mongo-driver/bson"
)

func MongoLoadMembersByRolePaginated(convID int64, role int, limit int64, offset int64) ([]conversation_models.MemberPayload, error) {
	filter := bson.M{"conversation_id": convID, "role": role}
	sort := bson.M{"joined_at": -1} // Du plus récent au plus ancien

	docs, err := Members.GetPaginated(filter, sort, offset, limit)
	if err != nil {
		return nil, err
	}

	var members []conversation_models.MemberPayload
	for _, doc := range docs {
		var m conversation_models.MemberPayload
		if err := pkg.ToStruct(doc, &m); err == nil {
			members = append(members, m)
		}
	}
	return members, nil
}
