package variables

// ############################################################################
// # PARAMÈTRES ET CONSTANTES DU DOMAINE POSTS
// ############################################################################

const (
	// Préfixe utilisé pour tagger automatiquement l'auteur dans ses propres publications
	AuthorTagPrefix = "user_"

	// Niveaux de permissions pour les identifications (tags) et mentions
	TagPermissionEveryone  = 0
	TagPermissionFollowers = 1
	TagPermissionFriends   = 2
	TagPermissionNobody    = 3

	// Niveaux de visibilité pour les posts
	PostVisibilityDeleted   = -1
	PostVisibilityAll       = 0
	PostVisibilitySubcriber = 1
	PostVisibilityFriend    = 2
)
