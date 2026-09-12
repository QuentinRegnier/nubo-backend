package search_models

import (
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
)

// CommunitySearchInput valide les paramètres de la requête GET.
// On utilise 'form' pour binder les query parameters (?q=...&limit=...).
type CommunitySearchInput struct {
	Prefix string `form:"q" json:"q" binding:"required"`
	Limit  int64  `form:"limit,default=20" json:"limit" binding:"omitempty,min=1,max=50"`
}

// CommunitySearchOutput est la réponse riche renvoyée au client mobile.
// On réutilise la structure CommunityLiteView existante.
type CommunitySearchOutput struct {
	Communities []conversation_models.CommunityLiteView `json:"communities"`
}
