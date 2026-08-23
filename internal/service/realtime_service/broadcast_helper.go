package realtime_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// BroadcastToConversation est le routeur intelligent qui décide du mode (Fan-Out vs Twitch)
func BroadcastToConversation(ctx context.Context, convID int64, eventType string, payload any) error {
	conv, err := object_cache_service.GetConversationFromObjectCache(ctx, convID)
	if err != nil {
		return err
	}

	// Mode Twitch (Communautés)
	if conv.Type == 2 || conv.Type == 3 {
		return DistributeToCommunity(ctx, eventType, payload, convID)
	}

	// Mode Fan-Out : on récupère les membres instantanément en RAM (O(1))
	membersStr, err := redis.ConvParticipants.SMembers(ctx, convID)
	if err != nil || len(membersStr) == 0 {
		return nil
	}

	var userIDs []int64
	for _, m := range membersStr {
		if id, err := strconv.ParseInt(m, 10, 64); err == nil {
			userIDs = append(userIDs, id)
		}
	}

	return DistributeToUsers(ctx, eventType, payload, userIDs)
}
