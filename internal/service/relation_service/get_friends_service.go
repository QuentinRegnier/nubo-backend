package relation_service

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DES AMIS
// ############################################################################

// GetFriends récupère la liste des amis d'un utilisateur cible.
func GetFriends(ctx context.Context, callerID int64, input relation_models.GetFriendsInput) (relation_models.GetFriendsOutput, error) {
	var errOffset, errLimit numan_error.Error
	input.Offset, errOffset, input.Limit, errLimit = pkg.BatchVerif(input.Offset, input.Limit)
	if errOffset != nil || errLimit != nil {
		return relation_models.GetFriendsOutput{}, numan_error.Combine(errOffset, errLimit)
	}

	// On cherche les relations entrantes (incoming) de type Ami.
	friendsViews, errFetch := fetchRelationsHydrated(ctx, callerID, input.TargetID, variables.RelationStateFriend, "incoming", input.Limit, input.Offset)
	if errFetch != nil {
		return relation_models.GetFriendsOutput{}, numan_error.NewInternal()
	}

	return relation_models.GetFriendsOutput{
		Users: friendsViews,
	}, nil
}
