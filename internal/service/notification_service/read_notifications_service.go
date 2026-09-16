package notification_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
)

// MarkNotificationsAsRead met à jour le curseur de lecture de l'utilisateur en O(1)
func MarkNotificationsAsRead(ctx context.Context, userID int64, input notification_models.ReadNotificationsInput) (notification_models.ReadNotificationsOutput, error) {
	// 1. Sauvegarde instantanée du curseur en RAM (L1) via un entier brut
	err := redis.NotificationCursors.SetPrimitive(ctx, userID, input.ReadUpToID)
	if err != nil {
		return notification_models.ReadNotificationsOutput{}, err
	}

	// 2. TEMPS RÉEL MULTI-DEVICE
	payload := map[string]any{
		"read_up_to_id": input.ReadUpToID,
		"read_at":       time.Now().UTC().Format(time.RFC3339),
	}

	// On capture l'erreur du broadcast sans la retourner immédiatement
	broadcastErr := realtime_service.DistributeToUsers(ctx, "notification.read", payload, []int64{userID})

	// ========================================================================
	// 3. MARQUAGE DU TEMPS (DIRTY FLAG)
	// ========================================================================
	// Placé à la fin absolue de la fonction pour garantir l'ordre temporel
	timestampMs := cache_service.TouchActivityTimestamp(ctx, userID)

	return notification_models.ReadNotificationsOutput{
		ActivityUpdateAt: domain.TimeToMillis(time.UnixMilli(timestampMs)),
	}, broadcastErr
}
