package post_models

// GetUserPostsInput valide les paramètres de la requête de profil
type GetUserPostsInput struct {
	CallerID     int64 `json:"-"`
	TargetUserID int64 `json:"user_id" binding:"required"`
	Limit        int64 `json:"limit"`
	Offset       int64 `json:"offset"`
	Force        bool  `json:"force"`
}
