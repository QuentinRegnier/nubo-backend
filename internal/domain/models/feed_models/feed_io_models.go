package feed_models

import "github.com/QuentinRegnier/nubo-backend/internal/domain/models/post_models"

// GetFeedInput valide la requête de scroll ou de refresh
type GetFeedInput struct {
	UserID        int64 `json:"user_id"`
	Force         bool  `json:"force"` // Détecté via le suffixe /force
	LastSeenIndex int   `form:"last_seen_index,default=0"`
}

// GetFeedOutput est la réponse riche renvoyée au client mobile
type GetFeedOutput struct {
	Status        string                      `json:"status"`          // Message de confirmation
	ActiveFeed    string                      `json:"active_feed"`     // Indique "A", "B", ou "C"
	LastSeenIndex int                         `json:"last_seen_index"` // Le nouveau curseur du client
	Posts         []post_models.GetPostOutput `json:"posts"`           // La donnée riche (Hydratée, signée, filtrée)
}
