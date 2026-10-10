package security_models

type RefreshJWTResponse struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}
