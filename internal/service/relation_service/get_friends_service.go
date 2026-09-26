package relation_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : RÉCUPÉRATION DES AMIS
// ############################################################################

// GetFriends récupère la liste des amis d'un utilisateur cible.
func GetFriends(ctx context.Context, callerID int64, input relation_models.GetFriendsInput) (relation_models.GetFriendsOutput, error) {

	// On cherche les relations entrantes (incoming) de type Ami.
	friendsViews, errFetch := fetchRelationsHydrated(ctx, callerID, input.TargetID, variables.RelationStateFriend, "incoming", input.Limit, input.Offset)
	if errFetch != nil {
		return relation_models.GetFriendsOutput{}, nubo_error.NewInternal()
	}

	return relation_models.GetFriendsOutput{
		Users: friendsViews,
	}, nil
}
