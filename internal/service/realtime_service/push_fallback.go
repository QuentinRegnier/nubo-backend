package realtime_service

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
)

type PushJob struct {
	UserID    int64  `json:"user_id"`
	EventType string `json:"event_type"`
	Payload   any    `json:"payload"`
}

func sendPushFallback(ctx context.Context, targetUserID int64, eventType string, payload any) {
	settings, err := object_cache_service.GetUserSettingsCascade(ctx, targetUserID)
	if err == nil {
		if !settings.Notifications.MasterPushEnabled {
			return
		}
		if strings.HasPrefix(eventType, "message.") && !settings.Notifications.NotifyMessages {
			return
		}
		if strings.HasPrefix(eventType, "notification.") {
			subType := strings.TrimPrefix(eventType, "notification.")
			if subType == "post_liked" && !settings.Notifications.NotifyLikes {
				return
			}
			if subType == "comment_added" && !settings.Notifications.NotifyComments {
				return
			}
		}
	}

	job := PushJob{
		UserID:    targetUserID,
		EventType: eventType,
		Payload:   payload,
	}

	bytes, err := json.Marshal(job)
	if err != nil {
		return
	}

	// APPEL PUR AU REPOSITORY (Collection WorkerQueue avec ID "firebase")
	err = redis.WorkerQueue.LPush(ctx, "firebase", bytes)
	if err == nil {
		log.Printf("🔔 Push en attente d'expédition vers Firebase (User: %d, Event: %s)", targetUserID, eventType)
	}
}
