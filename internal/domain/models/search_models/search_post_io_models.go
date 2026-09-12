package search_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"

// SearchPostInput valide les critères de recherche de publications.
type SearchPostInput struct {
	Query  string `json:"query" binding:"required,min=1"`
	Filter string `json:"filter" binding:"omitempty,oneof=views likes comments recent oldest trend"`
	Limit  int64  `json:"limit" binding:"omitempty,min=1,max=50"`
	Offset int64  `json:"offset" binding:"omitempty,min=0"`
}

// SearchPostOutput structure la réponse de l'API.
type SearchPostOutput struct {
	Posts                   []post_models.GetPostOutput `json:"posts"`
	HasTrendFilterAvailable bool                        `json:"has_trend_filter_available"`
}
