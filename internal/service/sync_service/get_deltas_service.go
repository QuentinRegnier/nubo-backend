package sync_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
)

// GetDeltas interroge le Ledger de l'utilisateur pour récupérer toutes les conversations
// ayant subi une mutation depuis le timestamp fourni.
func GetDeltas(ctx context.Context, callerID int64, input sync_models.GetDeltasInput) (sync_models.GetDeltasOutput, error) {

	// DÉLÉGATION AU CACHE SERVICE (Qui lui-même appelle le Repository Redis abstrait)
	convIDs, err := cache_service.GetModifiedConversationIDs(ctx, callerID, input.SinceMs)
	if err != nil {
		return sync_models.GetDeltasOutput{}, nubo_error.NewInternal(err)
	}

	// Prévention du retour 'null' en JSON si aucune conversation n'a muté
	if convIDs == nil {
		convIDs = make([]int64, 0)
	}

	return sync_models.GetDeltasOutput{
		ModifiedConversationIDs: convIDs,
	}, nil
}
