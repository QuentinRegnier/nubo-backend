package search_models

// AutocompleteTextInput valide le JSON entrant pour la barre de recherche.
type AutocompleteTextInput struct {
	Query string `json:"query" binding:"required,min=1"`
	Limit int64  `json:"limit" binding:"omitempty,min=1,max=50"`
}

// AutocompleteTextOutput unifie les résultats lexicographiques (0 I/O base de données)
type AutocompleteTextOutput struct {
	Users       UserSearchOutput      `json:"users"`
	Communities CommunitySearchOutput `json:"communities"`
}
