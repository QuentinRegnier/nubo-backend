package object_cache_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/vmihailenco/msgpack/v5"
)

func SetNotificationInObjectCache(ctx context.Context, notif notification_models.NotificationPayload) error {
	return redis.NotificationsObj.SetObject(ctx, notif.ID, notif)
}

// GetNotificationsView : Le Pipeline d'Hydratation L1 -> L2
// Renvoie strictement les Payloads bruts (La couche haute s'occupera de la View).
func GetNotificationsView(ctx context.Context, ids []int64) ([]notification_models.NotificationPayload, error) {
	if len(ids) == 0 {
		return []notification_models.NotificationPayload{}, nil
	}

	finalNotifs := make([]notification_models.NotificationPayload, 0, len(ids))
	tempMap := make(map[int64]notification_models.NotificationPayload)

	// 1. TENTATIVE L1 (MGET)
	result, err := redis.NotificationsObj.GetMany(ctx, ids)
	if err != nil {
		result = &redis.GetManyResult{MissingIDs: ids}
	} else {
		for id, data := range result.Found {
			var n notification_models.NotificationPayload
			if errDecode := msgpack.Unmarshal(data, &n); errDecode == nil {
				tempMap[id] = n
			} else {
				result.MissingIDs = append(result.MissingIDs, id)
			}
		}
	}

	// 2. FALLBACK L2 (MONGO)
	if len(result.MissingIDs) > 0 {
		mongoNotifs, err := mongo.MongoLoadNotificationsByIDs(result.MissingIDs)
		if err == nil {
			for _, n := range mongoNotifs {
				tempMap[n.ID] = n
				// PROMOTION L2 -> L1 (Auto-guérison)
				go func(notif notification_models.NotificationPayload) {
					_ = SetNotificationInObjectCache(context.Background(), notif)
				}(n)
			}
		}
	}

	// 3. ASSEMBLAGE
	for _, id := range ids {
		if n, ok := tempMap[id]; ok {
			finalNotifs = append(finalNotifs, n)
		}
	}

	return finalNotifs, nil
}
