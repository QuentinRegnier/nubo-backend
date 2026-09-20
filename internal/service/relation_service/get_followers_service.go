package relation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
)

func GetFollows(ctx context.Context, callerID int64, input relation_models.GetFollowersInput) (relation_models.GetFollowersOutput, error) {
	users, err := FetchRelationsHydrated(ctx, callerID, input.TargetID, 1, "incoming", input.Limit, input.Offset)
	if err != nil {
		return relation_models.GetFollowersOutput{}, err
	}
	return relation_models.GetFollowersOutput{Users: users}, nil
}
