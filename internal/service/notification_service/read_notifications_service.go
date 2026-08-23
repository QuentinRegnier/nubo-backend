package notification_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
)

// MarkNotificationsAsRead met à jour le curseur de lecture de l'utilisateur en O(1)
func MarkNotificationsAsRead(ctx context.Context, userID int64, input notification_models.ReadNotificationsInput) error {
	// 1. Sauvegarde instantanée du curseur en RAM (L1) via un entier brut
	// La clé sera "notifications:cursor:<userID>" avec pour valeur le Snowflake ID max lu
	err := redis.NotificationCursors.SetPrimitive(ctx, userID, input.ReadUpToID)
	if err != nil {
		return err
	}

	// 2. TEMPS RÉEL MULTI-DEVICE (Le fameux WebSocket dont on parlait)
	// On prévient tous les autres appareils connectés de l'utilisateur (ex: son PC et son iPhone)
	// pour qu'ils effacent la pastille rouge instantanément sans recharger la page.
	payload := map[string]any{
		"read_up_to_id": input.ReadUpToID,
		"read_at":       time.Now().UTC().Format(time.RFC3339),
	}

	return realtime_service.DistributeToUsers(ctx, "notification.read", payload, []int64{userID})
}
