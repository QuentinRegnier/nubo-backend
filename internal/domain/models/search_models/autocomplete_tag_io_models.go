package search_models

// AutocompleteTagInput valide le JSON entrant pour la recherche de tags.
type AutocompleteTagInput struct {
	Query  string `json:"query" binding:"required,min=1"`
	Offset int64  `form:"offset" json:"offset" binding:"required"`
	Limit  int64  `json:"limit" binding:"omitempty,min=1,max=50"`
}

// AutocompleteTagOutput unifie les résultats lexicographiques.
type AutocompleteTagOutput struct {
	Tags []string `json:"tags"`
}
