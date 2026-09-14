package realtime_service

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

type WSEvent struct {
	EventType string `json:"event_type"`
	Payload   any    `json:"payload"`
}

func DistributeToUsers(ctx context.Context, eventType string, payload any, userIDs []int64) error {
	if len(userIDs) == 0 {
		return nil
	}

	event := WSEvent{
		EventType: eventType,
		Payload:   payload,
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Appel pur du Repository Redis (Zéro infrastructure/push ici, juste du temps réel !)
	return redis.ChannelUser.PublishMultiple(ctx, userIDs, eventBytes)
}

func DistributeToCommunity(ctx context.Context, eventType string, payload any, communityID int64) error {
	event := WSEvent{
		EventType: eventType,
		Payload:   payload,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Appel pur du Repository Redis
	return redis.ChannelCommunity.Publish(ctx, communityID, eventBytes)
}
