package nubo_error

import (
	"errors"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

// PublicErrorResponse est le format standardisé renvoyé au client HTTP/Web.
// On conserve la clé "nubo_error" pour ne pas casser la rétrocompatibilité avec tes applications mobiles actuelles.
type PublicErrorResponse struct {
	Error   string `json:"nubo_error"`
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
			logger.Log.Error().Err(appErr.Err).Str("trace_id", traceID).Str("code", appErr.Code).Msg(appErr.Message)
		} else {
			logger.Log.Warn().Err(appErr.Err).Str("trace_id", traceID).Str("code", appErr.Code).Msg(appErr.Message)
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
	logger.Log.Error().Err(err).Str("trace_id", traceID).Msg("Unhandled internal error (fuite bas niveau détectée)")

	// ... mais on masque totalement l'erreur au client.
	c.AbortWithStatusJSON(http.StatusInternalServerError, PublicErrorResponse{
		Error:   "Une erreur interne est survenue. Nos équipes ont été alertées.",
		Code:    CodeInternalError,
		TraceID: traceID,
	})
}
