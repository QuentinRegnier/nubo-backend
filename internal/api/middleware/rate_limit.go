package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	// Import correct de ton infrastructure Redis
	redisInfra "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
)

const (
	// Limite : 50 requêtes maximum
	RateLimitMaxRequests = 50
	// Fenêtre de temps : 10 secondes
	RateLimitWindow = 10 * time.Second
)

// RateLimiter empêche les attaques DDoS applicatives
func RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// On identifie l'utilisateur par son IP
		clientIP := c.ClientIP()

		// Clé Redis unique pour cette IP et cette fenêtre de temps
		key := fmt.Sprintf("rate_limit:ip:%s", clientIP)

		ctx := context.Background()

		// CORRECTION : Utilisation de `Rdb` qui est ta vraie variable dans client.go
		count, err := redisInfra.Rdb.Incr(ctx, key).Result()
		if err != nil {
			// Fail-Open : on laisse passer si Redis plante pour ne pas bloquer l'API
			c.Next()
			return
		}

		// Si c'est la première requête, on définit le TTL de 10 secondes
		if count == 1 {
			redisInfra.Rdb.Expire(ctx, key, RateLimitWindow)
		}

		// Si on dépasse la limite
		if count > RateLimitMaxRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please calm down.",
			})
			return
		}

		c.Next()
	}
}
