package search_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"

// UserSearchInput valide les paramètres de la requête GET.
// On utilise 'form' pour binder les query parameters (?q=...&limit=...).
type UserSearchInput struct {
	Prefix string `form:"q" json:"q" binding:"required"`
	Limit  int64  `form:"limit,default=20" json:"limit" binding:"omitempty,min=1,max=50"`
}

// UserSearchOutput est la réponse riche renvoyée au client mobile.
// On réutilise la structure UserLiteView existante.
type UserSearchOutput struct {
	Users []auth_models.UserLiteView `json:"users"`
}
