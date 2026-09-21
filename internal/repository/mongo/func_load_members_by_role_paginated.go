package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"go.mongodb.org/mongo-driver/bson"
)

func MongoLoadMembersByRolePaginated(convID int64, role int, limit int64, offset int64) ([]member_models.MemberPayload, error) {
	filter := bson.M{"conversation_id": convID, "role": role}
	sort := bson.M{"joined_at": -1} // Du plus récent au plus ancien

	docs, err := Members.GetPaginated(filter, sort, offset, limit)
	if err != nil {
		return nil, err
	}

	var members []member_models.MemberPayload
	for _, doc := range docs {
		var m member_models.MemberPayload
		if err := pkg.ToStruct(doc, &m); err == nil {
			members = append(members, m)
		}
	}
	return members, nil
}
