package relation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DES ABONNÉS (FOLLOWERS)
// ############################################################################

// GetFollows récupère la liste des abonnés d'un utilisateur cible.
func GetFollows(ctx context.Context, callerID int64, input relation_models.GetFollowersInput) (relation_models.GetFollowersOutput, error) {

	// On cherche les relations entrantes (incoming) de type Abonnement.
	followersViews, errFetch := fetchRelationsHydrated(ctx, callerID, input.TargetID, variables.RelationStateFollow, "incoming", input.Limit, input.Offset)
	if errFetch != nil {
		return relation_models.GetFollowersOutput{}, nubo_error.NewInternal()
	}

	return relation_models.GetFollowersOutput{
		Users: followersViews,
	}, nil
}
