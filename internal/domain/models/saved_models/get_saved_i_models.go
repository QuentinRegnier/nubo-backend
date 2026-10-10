package saved_models

// GetSavedInput valide les paramètres de pagination dans l'URL (Query String)
type GetSavedInput struct {
	Limit  int64 `form:"limit,default=50" binding:"min=1,max=100"`
	Offset int64 `form:"offset,default=0" binding:"min=0"`
}
