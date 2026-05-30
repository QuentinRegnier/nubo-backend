package security_models

type RefreshMasterInput struct {
	UserID      int64  `json:"id_user" binding:"required"`
	MasterToken string `json:"master_token" binding:"required"` // L'ancien MasterToken
	Username    string `json:"username" binding:"required"`     // Le username de l'utilisateur
}
type RefreshMasterResponse struct {
	MasterToken string `json:"master_token"`
	Token       string `json:"token"` // Le nouveau JWT
	Message     string `json:"message"`
}
