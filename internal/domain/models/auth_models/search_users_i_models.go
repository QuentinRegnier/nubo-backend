package auth_models

type UserSearchInput struct {
	Prefix string `json:"q" binding:"required"`
	Limit  int64  `json:"limit" binding:"omitempty,min=1,max=50"`
}
