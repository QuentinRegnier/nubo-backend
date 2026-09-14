package cache_service

import (
	"context"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// AddNotificationToZSET plafonne à 100 éléments via Lua et rafraîchit le TTL.
func AddNotificationToZSET(ctx context.Context, userID int64, notifID int64, timestampMs int64) error {
	err := redis.NotificationsZSet.ZAddWithCap(ctx, userID, float64(timestampMs), strconv.FormatInt(notifID, 10), 100)
	_ = redis.NotificationsZSet.RefreshTTL(ctx, userID)
	return err
}

func GetNotificationIDsFromZSET(ctx context.Context, userID int64, offset int64, limit int64) ([]int64, error) {
	idStrings, err := redis.NotificationsZSet.ZRevRange(ctx, userID, offset, offset+limit-1)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for _, idStr := range idStrings {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	// On prolonge la vie du cache puisqu'il vient d'être accédé
	if len(ids) > 0 {
		_ = redis.NotificationsZSet.RefreshTTL(ctx, userID)
	}

	return ids, nil
}

func PurgeNotificationsZSET(ctx context.Context, userID int64) error {
	return redis.NotificationsZSet.DeleteObject(ctx, userID)
}

// TouchActivityTimestamp signale une modification dans l'état des notifications de l'utilisateur
// et retourne le timestamp exact (en millisecondes) généré par le serveur.
func TouchActivityTimestamp(ctx context.Context, userID int64) int64 {
	nowMs := time.Now().UnixMilli()

	// Il faut que tu t'assures d'avoir déclaré une collection Redis pour ça dans ton repository/redis
	// Exemple: var NotificationActivity = NewCollection("notification_activity", ...)
	_ = redis.NotificationActivity.SetPrimitive(ctx, userID, nowMs)

	return nowMs
}
