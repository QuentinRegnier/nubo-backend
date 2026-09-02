package middleware

import (
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/gin-gonic/gin"
)

// TraceIDMiddleware génère un Snowflake ID unique pour chaque requête entrante.
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Utilisation du moteur Snowflake (Génération en ~1ns, sans allocation BDD)
		traceID := strconv.FormatInt(pkg.GenerateID(), 10)

		// 1. Injection dans le contexte interne pour les handlers
		c.Set("trace_id", traceID)

		// 2. Injection dans les headers de retour pour le client (facilite le débuggage en prod)
		c.Header("X-Trace-ID", traceID)

		c.Next()
	}
}

// CustomRecoveryMiddleware remplace le gin.Recovery() basique.
// Il capture les crashs, extrait le TraceID, loggue la Stack Trace et renvoie une 500 JSON.
func CustomRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				traceID := c.GetString("trace_id")
				if traceID == "" {
					traceID = "unknown_panic"
				}

				// Capture de la Stack Trace complète
				stack := debug.Stack()

				// Log asynchrone et propre du crash
				logger.Log.Error().
					Str("trace_id", traceID).
					Interface("panic_reason", r).
					Bytes("stack_trace", stack).
					Msg("🔥 PANIC RECOVERED: Le serveur a empêché un crash total.")

				// Retour propre au client
				c.AbortWithStatusJSON(http.StatusInternalServerError, nubo_error.PublicErrorResponse{
					Error:   "Erreur critique du serveur.",
					Code:    nubo_error.CodeInternalError,
					TraceID: traceID,
				})
			}
		}()
		c.Next()
	}
}
