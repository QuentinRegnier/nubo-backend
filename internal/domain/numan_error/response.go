package numan_error

import (
	"errors"
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_log"
	"github.com/gin-gonic/gin"
)

// PublicErrorResponse est le format standardisé renvoyé au client HTTP/Web.
// On conserve la clé "numan_error" pour ne pas casser la rétrocompatibilité avec tes applications mobiles actuelles.
type PublicErrorResponse struct {
	Error   string `json:"numan_error"`
	Code    string `json:"code"`
	TraceID string `json:"trace_id"`
}

// RespondWithError analyse l'erreur, loggue la trace interne et renvoie le JSON propre.
func RespondWithError(c *gin.Context, err error) {
	// 1. Récupération du TraceID (généré par le middleware)
	traceID := c.GetString("trace_id")
	if traceID == "" {
		traceID = "unknown"
	}

	var appErr *AppError

	// 2. Si c'est une AppError (Erreur métier contrôlée)
	if errors.As(err, &appErr) {
		// Log intelligent : On ne fait sonner l'alarme (Error) que pour les 5xx.
		// Les erreurs clients (4xx comme un mauvais mot de passe) sont logguées en Warn.
		if appErr.HTTPStatus >= 500 {
			numan_log.Error(c).Err(appErr.Err).Str("trace_id", traceID).Str("code", appErr.Code).Msg(appErr.Message)
		} else {
			numan_log.Warn(c).Err(appErr.Err).Str("trace_id", traceID).Str("code", appErr.Code).Msg(appErr.Message)
		}

		c.AbortWithStatusJSON(appErr.HTTPStatus, PublicErrorResponse{
			Error:   appErr.Message,
			Code:    appErr.Code,
			TraceID: traceID,
		})
		return
	}

	// 3. Si c'est une erreur non gérée (Fuite bas niveau, ex: erreur SQL brute)
	// On loggue le vrai problème en interne...
	numan_log.Error(c).Err(err).Str("trace_id", traceID).Msg("Unhandled internal error (fuite bas niveau détectée)")

	// ... mais on masque totalement l'erreur au client.
	c.AbortWithStatusJSON(http.StatusInternalServerError, PublicErrorResponse{
		Error:   "Une erreur interne est survenue. Nos équipes ont été alertées.",
		Code:    CodeInternalError,
		TraceID: traceID,
	})
}
