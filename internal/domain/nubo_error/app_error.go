package nubo_error

import (
	"fmt"
	"net/http"
)

// AppError est notre erreur structurée de niveau GAFAM.
type AppError struct {
	HTTPStatus int    // Le code HTTP à renvoyer (ex: 404, 500)
	Code       string // Le code métier interne (ex: "USER_BANNED")
	Message    string // Le message public, "safe" pour l'utilisateur
	Err        error  // L'erreur originelle (interne) pour la stack trace et les logs
}

// Error implémente l'interface native 'error' de Go.
// C'est ce texte complet qui finira dans le fichier de log JSON (zéro fuite au client).
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

// --- DICTIONNAIRE DES CODES MÉTIER ---
const (
	CodeInternalError  = "INTERNAL_SERVER_ERROR"
	CodeNotFound       = "RESOURCE_NOT_FOUND"
	CodeForbidden      = "FORBIDDEN_ACCESS"
	CodeUnauthorized   = "UNAUTHORIZED"
	CodeInvalidPayload = "INVALID_PAYLOAD"
	CodeConflict       = "RESOURCE_CONFLICT"
)

// --- CONSTRUCTEURS SÉMANTIQUES ---

// NewInternal s'utilise quand la base de données ou un service tiers crashe.
// Le message public est volontairement générique pour ne pas fuiter d'infos.
func NewInternal(err error) *AppError {
	return &AppError{
		HTTPStatus: http.StatusInternalServerError,
		Code:       CodeInternalError,
		Message:    "Une erreur interne est survenue. Nos équipes ont été alertées.",
		Err:        err,
	}
}

// NewNotFound s'utilise quand une entité (Post, Utilisateur) n'existe pas.
func NewNotFound(code, message string, err error) *AppError {
	if code == "" {
		code = CodeNotFound
	}
	return &AppError{HTTPStatus: http.StatusNotFound, Code: code, Message: message, Err: err}
}

// NewForbidden s'utilise pour les règles métier (banni, pas les bons droits).
func NewForbidden(code, message string, err error) *AppError {
	if code == "" {
		code = CodeForbidden
	}
	return &AppError{HTTPStatus: http.StatusForbidden, Code: code, Message: message, Err: err}
}

// NewBadRequest s'utilise pour les JSON mal formés ou les limites dépassées.
func NewBadRequest(code, message string, err error) *AppError {
	if code == "" {
		code = CodeInvalidPayload
	}
	return &AppError{HTTPStatus: http.StatusBadRequest, Code: code, Message: message, Err: err}
}

// NewConflict s'utilise quand une ressource existe déjà (doublon email, pseudo, téléphone, etc.).
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

// NewAppError permet de construire une erreur avec un code HTTP sur-mesure
// (Utile pour les Middlewares : ex 413 Payload Too Large, 429 Too Many Requests).
func NewAppError(httpStatus int, code, message string, err error) *AppError {
	return &AppError{
		HTTPStatus: httpStatus,
		Code:       code,
		Message:    message,
		Err:        err,
	}
}
