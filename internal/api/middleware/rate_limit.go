package middleware

import (
	"context"
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/gin-gonic/gin"
)

const (
	// Limite : 50 requêtes maximum
	RateLimitMaxRequests = 50
)

// RateLimiter empêche les attaques DDoS applicatives
func RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// On identifie l'utilisateur par son IP
		clientIP := c.ClientIP()

		ctx := context.Background()

		// Utilisation propre de la Collection RateLimits (DDD)
		count, err := redis.RateLimits.Incr(ctx, clientIP)
		if err != nil {
			// Fail-Open : on laisse passer si Redis plante pour ne pas bloquer l'API
			c.Next()
			return
		}

		// Si c'est la première requête, on rafraîchit le TTL (10s préconfiguré dans le manager)
		if count == 1 {
			_ = redis.RateLimits.RefreshTTL(ctx, clientIP)
		}

		// Si on dépasse la limite
		if count > RateLimitMaxRequests {
			errLimit := nubo_error.NewAppError(http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "Too many requests. Please calm down.", nil)
			nubo_error.RespondWithError(c, errLimit)
			c.Abort()
			return
		}

		c.Next()
	}
}
