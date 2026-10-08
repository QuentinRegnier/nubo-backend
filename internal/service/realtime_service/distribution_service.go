package realtime_service

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # DTO ET MÉTHODES DE DISTRIBUTION TEMPS RÉEL (REDIS PUB/SUB)
// ############################################################################

// wsEvent représente la structure générique JSON envoyée aux instances clientes WebSockets.
type wsEvent struct {
	EventType string `json:"event_type"`
	Payload   any    `json:"payload"`
}

// DistributeToUsers distribue un événement ciblé à un lot spécifique d'utilisateurs (Fan-Out).
func DistributeToUsers(ctx context.Context, eventType string, payload any, targetUserIDs []int64) error {
	if len(targetUserIDs) == 0 {
		return nil
	}

	websocketEvent := wsEvent{
		EventType: eventType,
		Payload:   payload,
	}

	serializedEvent, errMarshal := json.Marshal(websocketEvent)
	if errMarshal != nil {
		nubo_log.Error(ctx).Err(errMarshal).Str("event", eventType).Msg("Erreur lors de la sérialisation de l'événement WebSocket")
		return nubo_error.NewInternal()
	}

	// Appel pur du Repository Redis (Pub/Sub)
	errRedis := redis.ChannelUser.PublishMultiple(ctx, targetUserIDs, serializedEvent)
	if errRedis != nil {
		nubo_log.Warn(ctx).Err(errRedis).Msg("Échec de la publication multiple sur le ChannelUser Redis")
		return nubo_error.NewInternal()
	}

	return nil
}

// DistributeToCommunity distribue un événement global à une Room entière (Mode Twitch / Communauté).
func DistributeToCommunity(ctx context.Context, eventType string, payload any, communityID int64) error {
	websocketEvent := wsEvent{
		EventType: eventType,
		Payload:   payload,
	}

	serializedEvent, errMarshal := json.Marshal(websocketEvent)
	if errMarshal != nil {
		nubo_log.Error(ctx).Err(errMarshal).Str("event", eventType).Msg("Erreur lors de la sérialisation de l'événement WebSocket (Communauté)")
		return nubo_error.NewInternal()
	}

	// Appel pur du Repository Redis (Pub/Sub)
	errRedis := redis.ChannelCommunity.Publish(ctx, communityID, serializedEvent)
	if errRedis != nil {
		nubo_log.Warn(ctx).Err(errRedis).Int64("community_id", communityID).Msg("Échec de la publication sur le ChannelCommunity Redis")
		return nubo_error.NewInternal()
	}

	return nil
}
