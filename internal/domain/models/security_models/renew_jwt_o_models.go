package security_models

type RenewJWTResponse struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}
