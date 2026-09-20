package relation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
)

func GetBlockedUsers(ctx context.Context, callerID int64, input relation_models.GetBlockedInput) (relation_models.GetBlockedOutput, error) {
	users, err := FetchRelationsHydrated(ctx, callerID, callerID, -1, "outgoing", input.Limit, input.Offset)
	if err != nil {
		return relation_models.GetBlockedOutput{}, err
	}
	return relation_models.GetBlockedOutput{Users: users}, nil
}
