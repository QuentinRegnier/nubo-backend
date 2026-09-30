package nubo_log

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/rs/zerolog"
)

// enrichFromContext extrait intelligemment les variables de contexte (Gin ou Context standard)
// pour garantir la traçabilité sans avoir à les répéter à chaque log.
func enrichFromContext(ctx context.Context, e *zerolog.Event) *zerolog.Event {
	if ctx == nil {
		return e
	}

	// Extraction du TraceID pour l'observabilité distribuée
	if traceID, ok := ctx.Value(LogKeyTraceID).(string); ok && traceID != "" {
		e.Str(LogKeyTraceID, traceID)
	} else if traceID, ok := ctx.Value("trace_id").(string); ok && traceID != "" {
		// Fallback pour la compatibilité avec gin.Context.Set("trace_id")
		e.Str(LogKeyTraceID, traceID)
	}

	// Extraction de l'ID Utilisateur (Sécurité & Audit)
	if userID, ok := ctx.Value(LogKeyUserID).(int64); ok && userID != 0 {
		e.Int64(LogKeyUserID, userID)
	} else if userID, ok := ctx.Value("userID").(int64); ok && userID != 0 {
		e.Int64(LogKeyUserID, userID)
	}

	return e
}

// Info lance un événement de niveau INFO avec le contexte métier injecté.
func Info(ctx context.Context) *LogBuilder {
	return &LogBuilder{event: enrichFromContext(ctx, logger.Log.Info())}
}

// Error lance un événement de niveau ERROR avec le contexte métier injecté.
func Error(ctx context.Context) *LogBuilder {
	return &LogBuilder{event: enrichFromContext(ctx, logger.Log.Error())}
}

// Warn lance un événement de niveau WARN avec le contexte métier injecté.
func Warn(ctx context.Context) *LogBuilder {
	return &LogBuilder{event: enrichFromContext(ctx, logger.Log.Warn())}
}

// Debug lance un événement de niveau DEBUG avec le contexte métier injecté.
func Debug(ctx context.Context) *LogBuilder {
	return &LogBuilder{event: enrichFromContext(ctx, logger.Log.Debug())}
}

// Fatal lance un événement de niveau FATAL (provoque un os.Exit(1)) avec le contexte métier.
func Fatal(ctx context.Context) *LogBuilder {
	return &LogBuilder{event: enrichFromContext(ctx, logger.Log.Fatal())}
}
