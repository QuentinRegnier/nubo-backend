package notification_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetNotifications orchestre la cascade de lecture L1 -> L2 avec auto-guérison de la Boîte aux lettres
func GetNotifications(ctx context.Context, callerID int64, input notification_models.GetNotificationsInput) ([]notification_models.NotificationView, error) {
	// 0. MODE FORCE (Purge manuelle via PWA Pull-to-refresh)
	if input.Force {
		_ = cache_service.PurgeNotificationsZSET(ctx, callerID)
	}

	var notificationIDs []int64
	var err error

	// 1. TENTATIVE L1 (Index ZSET Redis)
	if input.Offset < 100 && !input.Force {
		notificationIDs, err = cache_service.GetNotificationIDsFromZSET(ctx, callerID, input.Offset, input.Limit)
	}

	// 2. CACHE MISS ou DÉPASSEMENT -> FALLBACK L2 (Mongo)
	if err != nil || len(notificationIDs) == 0 {
		mongoNotifs, errMongo := mongo.MongoLoadNotificationsPaginated(callerID, input.Limit, input.Offset)
		if errMongo != nil {
			return nil, errMongo
		}

		go func(notifs []notification_models.NotificationPayload, currentOffset int64) {
			bgCtx := context.Background()
			for _, n := range notifs {
				_ = object_cache_service.SetNotificationInObjectCache(bgCtx, n)
				if currentOffset < 100 {
					_ = cache_service.AddNotificationToZSET(bgCtx, n.UserID, n.ID, n.CreatedAt.UnixMilli())
				}
			}
		}(mongoNotifs, input.Offset)

		var views []notification_models.NotificationView
		for _, notif := range mongoNotifs {
			views = append(views, hydrateNotificationView(ctx, callerID, notif))
		}
		return views, nil
	}

	// 3. CACHE HIT -> MGET sur l'Object Cache L1 (Récupère les Payloads bruts)
	payloads, err := object_cache_service.GetNotificationsView(ctx, notificationIDs)
	if err != nil {
		return nil, err
	}

	// 4. HYDRATATION EN RAM (La fameuse boucle rapide en O(N))
	views := make([]notification_models.NotificationView, 0, len(payloads))
	for _, notif := range payloads {
		views = append(views, hydrateNotificationView(ctx, callerID, notif))
	}

	return views, nil
}
