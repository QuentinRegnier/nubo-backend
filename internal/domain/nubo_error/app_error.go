package nubo_error

import (
	"fmt"
	"net/http"
)

// AppError est notre erreur structurée de niveau GAFAM.
type AppError struct {
	HTTPStatus int    // Le code HTTP à renvoyer (ex: 404, 500)
	Code       string // Le code métier interne standardisé (ex: "USER_BANNED")
	Message    string // Le message public, "safe" pour l'utilisateur
	Err        error  // L'erreur originelle (interne) pour la stack trace et les logs
}

// Error implémente l'interface native 'error' de Go.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s -> %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap permet au moteur Go de remonter la chaîne d'erreurs (Error Wrapping).
func (e *AppError) Unwrap() error {
	return e.Err
}

// ############################################################################
// # DICTIONNAIRE UNIQUE DES CODES MÉTIERS (STRICTEMENT RÉSERVÉ)
// # Tout nouveau code doit être ajouté ici. Interdiction de "hardcoder" un string.
// ############################################################################
const (
	CodeInternalError      = "INTERNAL_SERVER_ERROR"
	CodeDatabaseError      = "DATABASE_ERROR"
	CodeCacheError         = "CACHE_ERROR"
	CodeNotFound           = "RESOURCE_NOT_FOUND"
	CodeForbidden          = "FORBIDDEN_ACCESS"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeInvalidPayload     = "INVALID_PAYLOAD"
	CodeConflict           = "RESOURCE_CONFLICT"
	CodeTooManyRequests    = "TOO_MANY_REQUESTS"
	CodePayloadTooLarge    = "PAYLOAD_TOO_LARGE"
	CodeUnsupportedMedia   = "UNSUPPORTED_MEDIA_TYPE"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// ############################################################################
// # CONSTRUCTEURS SÉMANTIQUES
// # Ces fonctions wrap l'erreur originelle pour ne jamais la fuiter au client.
// ############################################################################

// NewInternal s'utilise quand la base de données (Postgres, Mongo) ou Redis crashe.
// Le message public est volontairement générique. L'erreur brute est logguée.
func NewInternal() *AppError {
	return &AppError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       CodeInternalError,
		Message:    "Une erreur interne est survenue. Nos équipes ont été alertées.",
		Err:        nil,
	}
}

// NewNotFound s'utilise quand une entité (Post, Utilisateur) n'existe pas ou plus.
func NewNotFound(code, message string, err error) *AppError {
	if code == "" {
		code = CodeNotFound
	}
	return &AppError{HTTPStatus: http.StatusNotFound, Code: code, Message: message, Err: err}
}

// NewForbidden s'utilise pour les règles métier (banni, manque de droits).
func NewForbidden(code, message string, err error) *AppError {
	if code == "" {
		code = CodeForbidden
	}
	return &AppError{HTTPStatus: http.StatusForbidden, Code: code, Message: message, Err: err}
}

// NewBadRequest s'utilise pour les JSON mal formés, tailles dépassées, requêtes impossibles.
func NewBadRequest(code, message string, err error) *AppError {
	if code == "" {
		code = CodeInvalidPayload
	}
	return &AppError{HTTPStatus: http.StatusBadRequest, Code: code, Message: message, Err: err}
}

// NewConflict s'utilise quand une ressource existe déjà (doublon email, pseudo, etc.).
func NewConflict(code, message string, err error) *AppError {
	if code == "" {
		code = CodeConflict
	}
	return &AppError{HTTPStatus: http.StatusConflict, Code: code, Message: message, Err: err}
}

// NewUnauthorized s'utilise pour les problèmes d'authentification (Token invalide, manquant, expiré).
func NewUnauthorized(code, message string, err error) *AppError {
	if code == "" {
		code = CodeUnauthorized
	}
	return &AppError{HTTPStatus: http.StatusUnauthorized, Code: code, Message: message, Err: err}
}

// NewTooManyRequests s'utilise pour les Rate Limiters (Anti-Spam).
func NewTooManyRequests(message string, err error) *AppError {
	return &AppError{HTTPStatus: http.StatusTooManyRequests, Code: CodeTooManyRequests, Message: message, Err: err}
}

// NewAppError permet de construire une erreur avec un code HTTP sur-mesure pour les Middlewares.
func NewAppError(httpStatus int, code, message string, err error) *AppError {
	return &AppError{
		HTTPStatus: httpStatus,
		Code:       code,
		Message:    message,
		Err:        err,
	}
}
