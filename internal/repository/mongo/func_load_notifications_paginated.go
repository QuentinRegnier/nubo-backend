package mongo

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_log"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoLoadNotificationsPaginated récupère l'historique d'un utilisateur par lots (L2 Fallback absolu).
// Le tri se fait par ordre chronologique décroissant (les plus récentes en premier).
func MongoLoadNotificationsPaginated(c context.Context, userID int64, limit int64, offset int64) ([]notification_models.NotificationPayload, error) {
	filter := map[string]any{"user_id": userID}
	sort := map[string]any{"created_at": -1}

	docs, err := Notifications.GetPaginated(filter, sort, offset, limit)
	if err != nil {
		nubo_log.Error(c).Err(err).Msg("Erreur interne lors de l'exécution de l'opération")
		return nil, nubo_error.NewInternal()
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
