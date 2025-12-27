package domain

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrDesactivated       = errors.New("account deactivated")
	ErrBanned             = errors.New("account banned")
	ErrNotFound           = errors.New("resource not found")
	ErrAlreadyExists      = errors.New("resource already exists")
	ErrInvalidIPAddress   = errors.New("invalid IP address")
)

type ErrorResponse struct {
	Error string `json:"error" example:"Invalid JSON format"`
}
