package variables

// ############################################################################
// # PARAMÈTRES ET CONSTANTES DU DOMAINE RECHERCHE (SEARCH)
// ############################################################################

const (
	// Filtres de recherche textuelle pour les requêtes frontend
	SearchFilterViews    = "views"
	SearchFilterLikes    = "likes"
	SearchFilterComments = "comments"
	SearchFilterRecent   = "recent"
	SearchFilterOldest   = "oldest"
	SearchFilterTrend    = "trend"

	// Correspondances pour le tri (Order Mode) dans PostgreSQL
	OrderModeViews    = 0
	OrderModeLikes    = 1
	OrderModeComments = 2
	OrderModeRecent   = 3
	OrderModeOldest   = 4

	// Limites de sécurité par défaut pour la taille des listes retournées
	DefaultAutocompleteLimit = 15
	DefaultGlobalSearchLimit = 10
)
