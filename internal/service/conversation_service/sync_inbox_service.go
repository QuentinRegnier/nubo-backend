package conversation_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// SyncInbox vérifie le delta temporel en RAM (O(1)) et ne retourne l'Inbox hydratée que si nécessaire.
func SyncInbox(ctx context.Context, callerID int64, input conversation_models.SyncInboxInput) (conversation_models.SyncInboxOutput, error) {
	output := conversation_models.SyncInboxOutput{
		NeedUpdate: false,
	}

	// 1. Récupération instantanée du marqueur d'activité L1 (O(1))
	serverTimestamp, err := redis.InboxActivity.GetInt64(ctx, callerID)
	if err != nil || serverTimestamp == 0 {
		// Cas d'un cold start complet ou d'une purge Redis : on force un timestamp actuel
		serverTimestamp = time.Now().UnixMilli()
		_ = redis.InboxActivity.SetPrimitive(ctx, callerID, serverTimestamp)
	}

	// 2. Résolution du Delta Sync
	if input.ClientUpdatedAt >= serverTimestamp {
		// LE CLIENT EST À JOUR ! Zéro I/O supplémentaire.
		output.ServerUpdated = serverTimestamp
		return output, nil
	}

	// LE SERVEUR EST PLUS RÉCENT : On doit hydrater et renvoyer l'Inbox.
	output.NeedUpdate = true
	output.ServerUpdated = serverTimestamp

	// On demande par défaut les 50 premières conversations (Paramétrable si besoin)
	reqInput := conversation_models.GetUserConversationsInput{
		Limit:  50,
		Offset: 0,
		Force:  false,
	}

	// CORRECTION : Appel de la fonction "GetUserConversationsPaginated" (avec un "s")
	inboxData, err := GetUserConversationsPaginated(ctx, callerID, reqInput)
	if err != nil {
		return output, err
	}

	output.Conversations = inboxData.Conversations
	return output, nil
}
