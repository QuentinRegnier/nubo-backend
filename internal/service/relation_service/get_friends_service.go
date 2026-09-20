package relation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
)

func GetFriends(ctx context.Context, callerID int64, input relation_models.GetFriendsInput) (relation_models.GetFriendsOutput, error) {
	users, err := FetchRelationsHydrated(ctx, callerID, input.TargetID, 2, "incoming", input.Limit, input.Offset)
	if err != nil {
		return relation_models.GetFriendsOutput{}, err
	}
	return relation_models.GetFriendsOutput{Users: users}, nil
}
