package post_models

// GetUserPostsInput valide les paramètres de la requête de profil
type GetUserPostsInput struct {
	CallerID     int64 `json:"caller_id"` // Protégé par le JWT
	TargetUserID int64 `form:"user_id" binding:"required"`
	Limit        int64 `form:"limit,default=50"`
	Offset       int64 `form:"offset,default=0"`
	Force        bool  `form:"force"`
}
