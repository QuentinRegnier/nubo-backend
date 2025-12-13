package tools

import "errors"

var (
	ErrInvalidCredentials = errors.New("identifiants invalides")
	ErrNotFound           = errors.New("ressource non trouvée")
	ErrAlreadyExists      = errors.New("ressource déjà existante")
)

type ErrorResponse struct {
	Error string `json:"error" example:"Invalid JSON format"`
}
