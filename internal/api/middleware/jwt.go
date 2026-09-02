package middleware

import (
	"fmt"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			nubo_error.RespondWithError(c, nubo_error.NewUnauthorized("MISSING_TOKEN", "Authorization header manquant.", nil))
			c.Abort()
			return
		}

		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		keyFunc := func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("algorithme JWT invalide")
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		}

		token, err := jwt.Parse(tokenString, keyFunc)
		if err != nil || !token.Valid {
			nubo_error.RespondWithError(c, nubo_error.NewUnauthorized("INVALID_TOKEN", "Token invalide.", err))
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			nubo_error.RespondWithError(c, nubo_error.NewUnauthorized("INVALID_CLAIMS", "Claims invalides.", nil))
			c.Abort()
			return
		}

		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().After(time.Unix(int64(exp), 0)) {
				nubo_error.RespondWithError(c, nubo_error.NewUnauthorized("EXPIRED_TOKEN", "Token expiré.", nil))
				c.Abort()
				return
			}
		}

		// EXTRACTION DES DONNÉES CLÉS
		c.Set("userID", claims["sub"]) // ID de l'utilisateur

		if dev, ok := claims["dev"].(string); ok {
			c.Set("firebaseInstallationID", dev) // Identifiant unique de la session
		} else {
			nubo_error.RespondWithError(c, nubo_error.NewUnauthorized("OBSOLETE_TOKEN", "Token format obsolete (missing device info).", nil))
			c.Abort()
			return
		}

		c.Next()
	}
}
