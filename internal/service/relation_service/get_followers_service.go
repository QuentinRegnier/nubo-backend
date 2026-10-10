package relation_service

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DES ABONNÉS (FOLLOWERS)
// ############################################################################

// GetFollows récupère la liste des abonnés d'un utilisateur cible.
func GetFollows(ctx context.Context, callerID int64, input relation_models.GetFollowersInput) (relation_models.GetFollowersOutput, error) {
	var errOffset, errLimit numan_error.Error
	input.Offset, errOffset, input.Limit, errLimit = pkg.BatchVerif(input.Offset, input.Limit)
	if errOffset != nil || errLimit != nil {
		return relation_models.GetFollowersOutput{}, numan_error.Combine(errOffset, errLimit)
	}

	// On cherche les relations entrantes (incoming) de type Abonnement.
	followersViews, errFetch := fetchRelationsHydrated(ctx, callerID, input.TargetID, variables.RelationStateFollow, "incoming", input.Limit, input.Offset)
	if errFetch != nil {
		return relation_models.GetFollowersOutput{}, numan_error.NewInternal()
	}

	return relation_models.GetFollowersOutput{
		Users: followersViews,
	}, nil
}
