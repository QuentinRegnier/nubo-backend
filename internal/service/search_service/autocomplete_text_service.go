package search_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/search_models"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # SERVICE : AUTOCOMPLÉTION GLOBALE (UTILISATEURS & COMMUNAUTÉS)
// ############################################################################

// AutocompleteText orchestre l'autocomplétion globale du moteur de recherche
// en invoquant en parallèle les services ultra-rapides du domaine.
func AutocompleteText(ctx context.Context, callerID int64, input search_models.AutocompleteTextInput) (search_models.AutocompleteTextOutput, error) {

	autocompleteOutput := search_models.AutocompleteTextOutput{}

	searchLimit := input.Limit
	if searchLimit == 0 {
		searchLimit = variables.DefaultGlobalSearchLimit
	}

	// ── ÉTAPE 1 : RECHERCHE DES UTILISATEURS (CACHE L1) ─────────────────────

	usersSearchInput := search_models.UserSearchInput{
		Prefix: input.Query,
		Limit:  searchLimit,
	}

	// Erreur volontairement ignorée ici pour qu'un fail sur les utilisateurs
	// ne fasse pas crasher la requête pour les communautés.
	usersOutput, _ := SearchUsers(ctx, callerID, usersSearchInput)
	autocompleteOutput.Users = usersOutput

	// ── ÉTAPE 2 : RECHERCHE DES COMMUNAUTÉS (CACHE L1) ──────────────────────

	communitiesSearchInput := search_models.CommunitySearchInput{
		Prefix: input.Query,
		Limit:  searchLimit,
	}

	communitiesOutput, _ := SearchCommunities(ctx, callerID, communitiesSearchInput)
	autocompleteOutput.Communities = communitiesOutput

	// Note Architecturale : Pas de recherche de tags/posts ici car la timeline
	// possède ses propres onglets dédiés avec filtres asynchrones.
	return autocompleteOutput, nil
}
