package notification_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

// GetNotifications orchestre la cascade de lecture L1 -> L2 avec auto-guérison de la Boîte aux lettres
func GetNotifications(ctx context.Context, callerID int64, input notification_models.GetNotificationsInput) ([]notification_models.NotificationPayload, error) {
	// 0. MODE FORCE (Purge manuelle via PWA Pull-to-refresh)
	if input.Force {
		_ = cache_service.PurgeNotificationsZSET(ctx, callerID)
	}

	var notificationIDs []int64
	var err error

	// 1. TENTATIVE L1 (Index ZSET Redis)
	// Si on dépasse 100, on est hors de la limite du Capping (L1), on bypass silencieusement
	if input.Offset < 100 && !input.Force {
		notificationIDs, err = cache_service.GetNotificationIDsFromZSET(ctx, callerID, input.Offset, input.Limit)
	}

	// 2. CACHE MISS ou DÉPASSEMENT -> FALLBACK L2 (Mongo)
	if err != nil || len(notificationIDs) == 0 {
		// On charge les objets complets depuis le stockage à froid
		mongoNotifs, errMongo := mongo.MongoLoadNotificationsPaginated(callerID, input.Limit, input.Offset)
		if errMongo != nil {
			return nil, errMongo
		}

		// Rétro-Hydratation asynchrone (Auto-guérison du Cache L1)
		go func(notifs []notification_models.NotificationPayload, currentOffset int64) {
			bgCtx := context.Background()
			for _, n := range notifs {
				// A. Hydratation de l'Object Cache (JSON)
				_ = object_cache_service.SetNotificationInObjectCache(bgCtx, n)

				// B. Hydratation du ZSET (Uniquement si on est dans les 100 premiers)
				if currentOffset < 100 {
					_ = cache_service.AddNotificationToZSET(bgCtx, n.UserID, n.ID, n.CreatedAt.UnixMilli())
				}
			}
		}(mongoNotifs, input.Offset)

		return mongoNotifs, nil
	}

	// 3. CACHE HIT -> HYDRATATION (MGET sur l'Object Cache L1)
	return object_cache_service.GetNotificationsView(ctx, notificationIDs)
}
