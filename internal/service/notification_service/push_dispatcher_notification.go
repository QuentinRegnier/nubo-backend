package notification_service

import (
	"context"
	"encoding/json"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/realtime_service"
)

// Structure locale privée pour casser la dépendance cyclique vers le package worker
type pushJob struct {
	UserID    int64  `json:"user_id"`
	EventType string `json:"event_type"`
	Payload   any    `json:"payload"`
}

// DispatchNotification forge la notif, la sauvegarde (L1+L2), la diffuse en temps réel (WS),
// et l'envoie au Push Worker si les paramètres de l'utilisateur l'autorisent.
func DispatchNotification(ctx context.Context, targetUserID int64, actorID int64, eventType string, targetID int64) error {

	if targetUserID == actorID {
		return nil // Règle métier : On ne s'auto-notifie pas !
	}

	notif := notification_models.NotificationPayload{
		ID:        pkg.GenerateID(),
		UserID:    targetUserID,
		ActorID:   actorID,
		Type:      eventType,
		TargetID:  targetID,
		IsRead:    false,
		CreatedAt: domain.NowMillis(),
	}

	// 1. RAM L1 (JSON + Index ZSET plafonné à 100)
	_ = object_cache_service.SetNotificationInObjectCache(ctx, notif)
	_ = cache_service.AddNotificationToZSET(ctx, targetUserID, notif.ID, notif.CreatedAt)

	// 2. Persistance L2 (Mongo uniquement)
	_ = redis.EnqueueDB(ctx, notif.ID, targetUserID, redis.EntityNotification, redis.ActionCreate, notif, redis.TargetMongo)

	// 3. Temps Réel (WebSocket)
	notifView := hydrateNotificationView(ctx, targetUserID, notif)
	_ = realtime_service.DistributeToUsers(ctx, "notification."+eventType, notifView, []int64{targetUserID})

	// =====================================================================
	// MATRICE DE ROUTAGE DES NOTIFICATIONS PUSH
	// =====================================================================

	// 4. FILTRE DE PRÉSENCE (O(1))
	// Si l'utilisateur a l'application ouverte, le WebSocket a déjà fait le travail silencieusement.
	if cache_service.IsUserOnline(ctx, targetUserID) {
		return nil // On coupe ici pour le Push
	}

	// 5. RÉCUPÉRATION DES PARAMÈTRES (O(1) L1 -> L2 -> L3)
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, targetUserID)
	if err != nil {
		return err // Impossible de vérifier les droits, on refuse l'envoi par sécurité
	}

	// 6. MASTER SWITCH
	if !settings.Notifications.MasterPushEnabled {
		return nil // L'utilisateur a coupé toutes les notifications
	}

	// 7. MATRICE DE ROUTAGE GLOBALE
	canSendPush := false

	switch eventType {
	case "post_liked", "comment_liked":
		canSendPush = settings.Notifications.NotifyLikes
	case "comment_added":
		canSendPush = settings.Notifications.NotifyComments
	case "relation_followed":
		canSendPush = settings.Notifications.NotifyNewFollower
	case "friendship_established":
		canSendPush = settings.Notifications.NotifyFriendRequest
	case "group_invited":
		canSendPush = settings.Notifications.NotifyGroupInvites
	// Les cas "message" et "mention" ne sont PAS ici car gérés par le push_dispatcher de la messagerie
	default:
		canSendPush = true // Fallback pour les alertes systèmes vitales
	}

	// 8. EXPÉDITION AU WORKER FCM
	if canSendPush {
		job := pushJob{
			UserID:    targetUserID,
			EventType: eventType,
			Payload: map[string]interface{}{
				"actor_id":  actorID,
				"target_id": targetID,
			},
		}

		if jobBytes, err := json.Marshal(job); err == nil {
			// APPEL PUR AU REPOSITORY (Collection WorkerQueue avec l'ID "firebase")
			_ = redis.WorkerQueue.LPush(ctx, "firebase", jobBytes)
		}
	}

	return nil
}
