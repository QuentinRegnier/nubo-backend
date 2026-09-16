package notification_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
)

// SyncActivity vérifie le delta temporel et les IDs Snowflake pour ne renvoyer que les nouveautés.
func SyncActivity(ctx context.Context, callerID int64, input notification_models.SyncActivityInput) (notification_models.SyncActivityOutput, error) {
	output := notification_models.SyncActivityOutput{
		NeedUpdate: false,
	}

	// 1. Récupération des 100 dernières notifications via l'orchestrateur existant (L1 -> L2)
	// L'offset 0 et limit 100 garantissent qu'on a le sommet du ZSET chronologique.
	reqInput := notification_models.GetNotificationsInput{
		Limit:  100,
		Offset: 0,
		Force:  false,
	}

	notifs, err := GetNotifications(ctx, callerID, reqInput)
	if err != nil {
		return output, err
	}

	// Cas : Le centre de notifications est vide
	if len(notifs) == 0 {
		output.ServerUpdated = time.Now().UnixMilli()
		return output, nil
	}

	// 2. Date de référence serveur : La date de la notification la plus récente
	latestNotif := notifs[0]
	serverUpdated := latestNotif.CreatedAt
	output.ServerUpdated = serverUpdated

	// 3. Résolution du Delta Sync
	// Si la notification la plus récente est plus vieille ou égale à la date du client,
	// ET que son ID Snowflake n'est pas supérieur à ce que le client a déjà vu : Le client est à jour.
	if serverUpdated <= input.ClientUpdatedAt && latestNotif.ID <= input.ReadUpToID {
		return output, nil
	}

	// 4. Filtrage chirurgical O(N) en RAM
	// On conserve le type View ! Zéro ré-hydratation inutile.
	var newNotifs []notification_models.NotificationView

	for _, n := range notifs {
		if n.ID > input.ReadUpToID || n.CreatedAt > input.ClientUpdatedAt {
			newNotifs = append(newNotifs, n)
		} else {
			// Le ZSET est trié chronologiquement de manière décroissante.
			// Dès qu'on tombe sur une notification connue, on sait que toutes les suivantes le sont aussi.
			break
		}
	}

	// Si après filtrage, il n'y a rien de neuf, on sécurise le retour
	if len(newNotifs) == 0 {
		return output, nil
	}

	output.NeedUpdate = true
	output.Notifications = newNotifs // Assigation directe du tableau filtré

	return output, nil
}
