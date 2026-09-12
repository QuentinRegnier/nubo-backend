package notification_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
)

// DispatchNotification forge la notif, la sauvegarde (L1+L2) et la diffuse en temps réel
func DispatchNotification(ctx context.Context, ownerID, actorID int64, notifType string, targetID int64) error {
	if ownerID == actorID {
		return nil // Règle métier : On ne s'auto-notifie pas !
	}

	notif := notification_models.NotificationPayload{
		ID:        pkg.GenerateID(),
		UserID:    ownerID,
		ActorID:   actorID,
		Type:      notifType,
		TargetID:  targetID,
		IsRead:    false,
		CreatedAt: time.Now().UTC(),
	}

	// 1. RAM L1 (JSON + Index ZSET plafonné à 100)
	_ = object_cache_service.SetNotificationInObjectCache(ctx, notif)
	_ = cache_service.AddNotificationToZSET(ctx, ownerID, notif.ID, notif.CreatedAt.UnixMilli())

	// 2. Persistance L2 (Mongo uniquement, comme défini au Bloc 1)
	_ = redis.EnqueueDB(ctx, notif.ID, ownerID, redis.EntityNotification, redis.ActionCreate, notif, redis.TargetMongo)

	// 3. Temps Réel (WebSocket)
	// ✅ HYDRATATION EN VUE : On a un seul propriétaire, donc on peut générer l'URL HMAC en toute sécurité.
	notifView := hydrateNotificationView(ctx, ownerID, notif)

	return realtime_service.DistributeToUsers(ctx, "notification."+notifType, notifView, []int64{ownerID})
}
