package middleware

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header manquant"})
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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token invalide"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Claims invalides"})
			return
		}

		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().After(time.Unix(int64(exp), 0)) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token expiré"})
				return
			}
		}

		// EXTRACTION DES DONNÉES CLÉS
		c.Set("userID", claims["sub"]) // ID de l'utilisateur

		if dev, ok := claims["dev"].(string); ok {
			c.Set("deviceToken", dev) // Identifiant unique de la session
		} else {
			// Si c'est un vieux token sans claim 'dev', on rejette par sécurité
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token format obsolete (missing device info)"})
			return
		}

		c.Next()
	}
}
