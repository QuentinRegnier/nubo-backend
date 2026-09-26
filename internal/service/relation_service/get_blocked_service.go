package relation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DES UTILISATEURS BLOQUÉS
// ############################################################################

// GetBlockedUsers récupère la liste des utilisateurs que l'appelant a bloqués.
func GetBlockedUsers(ctx context.Context, callerID int64, input relation_models.GetBlockedInput) (relation_models.GetBlockedOutput, error) {

	// La cible est l'appelant lui-même, on cherche les relations sortantes (outgoing) bloquées.
	blockedUsersViews, errFetch := fetchRelationsHydrated(ctx, callerID, callerID, variables.RelationStateBlocked, "outgoing", input.Limit, input.Offset)
	if errFetch != nil {
		return relation_models.GetBlockedOutput{}, nubo_error.NewInternal()
	}

	return relation_models.GetBlockedOutput{
		Users: blockedUsersViews,
	}, nil
}
