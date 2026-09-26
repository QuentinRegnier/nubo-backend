package sync_service

import (
	"context"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
)

// ############################################################################
// # SERVICE : SYNCHRONISATION LÉGÈRE DE L'INBOX (DELTA SYNC)
// ############################################################################

// SyncInbox vérifie le delta temporel en RAM (O(1)) et ne retourne l'Inbox hydratée
// que si le serveur possède des données plus récentes que le client.
func SyncInbox(ctx context.Context, callerID int64, input sync_models.SyncInboxInput) (sync_models.SyncInboxOutput, error) {

	syncOutput := sync_models.SyncInboxOutput{
		NeedUpdate: false,
	}

	// ── ÉTAPE 1 : LECTURE DU MARQUEUR D'ACTIVITÉ L1 (O(1)) ──────────────────

	serverLatestActivityTimestamp, errRedis := redis.InboxActivity.GetInt64(ctx, callerID)

	if errRedis != nil || serverLatestActivityTimestamp == 0 {
		// Cas d'un cold start complet ou d'une purge volatile Redis :
		// On force la génération d'un timestamp actuel valide.
		serverLatestActivityTimestamp = time.Now().UnixMilli()
		_ = redis.InboxActivity.SetPrimitive(ctx, callerID, serverLatestActivityTimestamp)
	}

	// ── ÉTAPE 2 : ÉVALUATION DU DELTA (RÉSOLUTION DE CONFLIT) ───────────────

	if input.ClientUpdatedAt >= serverLatestActivityTimestamp {
		// LE CLIENT EST À JOUR ! Zéro I/O supplémentaire BDD/Cache nécessaire.
		syncOutput.ServerUpdated = serverLatestActivityTimestamp
		return syncOutput, nil
	}

	// ── ÉTAPE 3 : HYDRATATION EN CAS DE DÉSYNCHRONISATION ───────────────────

	syncOutput.NeedUpdate = true
	syncOutput.ServerUpdated = serverLatestActivityTimestamp

	// Requête par défaut pour la synchronisation à froid : les 50 premières conversations
	inboxRequestInput := conversation_models.GetUserConversationsInput{
		Limit:  50,
		Offset: 0,
		Force:  false,
	}

	inboxPaginatedData, errInbox := conversation_service.GetUserConversationsPaginated(ctx, callerID, inboxRequestInput)
	if errInbox != nil {
		logger.Log.Error().Err(errInbox).Int64("user_id", callerID).Msg("Erreur critique lors de la synchronisation de l'inbox.")
		return syncOutput, nubo_error.NewInternal() // Protection des détails d'infrastructure
	}

	syncOutput.Conversations = inboxPaginatedData.Conversations
	return syncOutput, nil
}
