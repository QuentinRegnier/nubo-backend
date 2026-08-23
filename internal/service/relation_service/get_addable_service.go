package relation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// GetAddableUsers délègue la résolution paginée au service de cache hybride
func GetAddableUsers(ctx context.Context, callerID int64, input relation_models.GetAddableInput) (relation_models.GetAddableOutput, error) {
	users, err := cache_service.GetAddableUsersFromSpeedCache(ctx, callerID, input.Limit, input.Offset, input.Force)
	if err != nil {
		return relation_models.GetAddableOutput{}, err
	}

	return relation_models.GetAddableOutput{Users: users}, nil
}
