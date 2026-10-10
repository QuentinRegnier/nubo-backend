package mongo

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
)

// MongoLoadNotificationsByIDs récupère un lot précis de notifications pour l'hydratation L1.
func MongoLoadNotificationsByIDs(c context.Context, ids []int64) ([]notification_models.NotificationPayload, error) {
	filter := map[string]any{"id": map[string]any{"$in": ids}}

	docs, err := Notifications.Get(filter, nil)
	if err != nil {
		numan_log.Error(c).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
		return nil, numan_error.NewInternal()
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
