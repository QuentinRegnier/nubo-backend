package search_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
)

// AutocompleteText orchestre l'autocomplétion en faisant appel aux services de domaine spécifiques
func AutocompleteText(ctx context.Context, callerID int64, input search_models.AutocompleteTextInput) (search_models.AutocompleteTextOutput, error) {
	output := search_models.AutocompleteTextOutput{}

	limit := input.Limit
	if limit == 0 {
		limit = 10
	}

	// 1. RECHERCHE DES UTILISATEURS via le service existant
	usersOutput, _ := SearchUsers(ctx, callerID, search_models.UserSearchInput{
		Prefix: input.Query,
		Limit:  limit,
	})
	output.Users = usersOutput

	// 2. RECHERCHE DES COMMUNAUTÉS via le service existant
	communitiesOutput, _ := SearchCommunities(ctx, callerID, search_models.CommunitySearchInput{
		Prefix: input.Query,
		Limit:  limit,
	})
	output.Communities = communitiesOutput

	// Pas de recherche de tags ici, car ce sera un onglet dédié comme expliqué.
	return output, nil
}
