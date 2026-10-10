package search_service

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
)

// ############################################################################
// # SERVICE : AUTOCOMPLÉTION GLOBALE (UTILISATEURS & COMMUNAUTÉS)
// ############################################################################

func AutocompleteText(ctx context.Context, callerID int64, input search_models.AutocompleteTextInput) (search_models.AutocompleteTextOutput, error) {

	autocompleteOutput := search_models.AutocompleteTextOutput{}

	var err_offset, errLimit numan_error.Error
	input.Offset, err_offset, input.Limit, errLimit = pkg.BatchVerif(input.Offset, input.Limit)
	if err_offset != nil || errLimit != nil {
		return search_models.AutocompleteTextOutput{}, numan_error.Combine(err_offset, errLimit)
	}

	// ── ÉTAPE 1 : RECHERCHE DES UTILISATEURS (CACHE L1) ─────────────────────
	usersSearchInput := search_models.UserSearchInput{
		Prefix: input.Query,
		Limit:  input.Limit,
		Offset: input.Offset, // NOUVEAU
	}

	usersOutput, _ := searchUsers(ctx, callerID, usersSearchInput)
	autocompleteOutput.Users = usersOutput

	// ── ÉTAPE 2 : RECHERCHE DES COMMUNAUTÉS (CACHE L1) ──────────────────────
	communitiesSearchInput := search_models.CommunitySearchInput{
		Prefix: input.Query,
		Limit:  input.Limit,
		Offset: input.Offset, // NOUVEAU
	}

	communitiesOutput, _ := searchCommunities(ctx, callerID, communitiesSearchInput)
	autocompleteOutput.Communities = communitiesOutput

	return autocompleteOutput, nil
}
