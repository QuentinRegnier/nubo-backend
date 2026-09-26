package sync_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// ############################################################################
// # SERVICE : DELTA SYNC DES CONVERSATIONS (LEDGER)
// ############################################################################

// GetDeltas interroge le Ledger granulaire SQLite (synchronisé via Redis)
// pour récupérer toutes les conversations ayant subi une mutation depuis le timestamp fourni.
func GetDeltas(ctx context.Context, callerID int64, input sync_models.GetDeltasInput) (sync_models.GetDeltasOutput, error) {

	// ── ÉTAPE 1 : DÉLÉGATION AU CACHE SERVICE (O(log N)) ────────────────────

	modifiedConversationIDs, errCache := cache_service.GetModifiedConversationIDs(ctx, callerID, input.SinceMs)
	if errCache != nil {
		logger.Log.Error().Err(errCache).Int64("user_id", callerID).Msg("Échec L1 lors de la récupération des deltas de conversation")
		return sync_models.GetDeltasOutput{}, nubo_error.NewInternal()
	}

	// ── ÉTAPE 2 : PRÉVENTION JSON (SÉCURITÉ NULL) ───────────────────────────

	if modifiedConversationIDs == nil {
		modifiedConversationIDs = make([]int64, 0)
	}

	return sync_models.GetDeltasOutput{
		ModifiedConversationIDs: modifiedConversationIDs,
	}, nil
}
