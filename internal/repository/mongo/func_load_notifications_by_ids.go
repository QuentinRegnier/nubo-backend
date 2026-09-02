package mongo

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoLoadNotificationsByIDs récupère un lot précis de notifications pour l'hydratation L1.
func MongoLoadNotificationsByIDs(ids []int64) ([]notification_models.NotificationPayload, error) {
	filter := map[string]any{"id": map[string]any{"$in": ids}}

	docs, err := Notifications.Get(filter, nil)
	if err != nil {
		return nil, nubo_error.NewInternal(err)
	}

	var notifs []notification_models.NotificationPayload
	for _, doc := range docs {
		var n notification_models.NotificationPayload
		if err := pkg.ToStruct(doc, &n); err == nil {
			notifs = append(notifs, n)
		}
	}
	return notifs, nil
}
