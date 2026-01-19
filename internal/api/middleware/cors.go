package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware gère les requêtes OPTIONS (Preflight) et les headers CORS
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Qui a le droit de nous appeler ?
		// "*" = tout le monde (pour le dev). En prod, mettre "https://mon-app-nubo.com"
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		// 2. Que permet-on de faire ? (Méthodes)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")

		// 3. Quels headers le client a le droit d'envoyer ? (CRUCIAL POUR NOUS)
		// Il faut lister TOUS tes headers de sécurité ici, sinon le navigateur les bloquera.
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, "+
				"X-Signature, X-Timestamp, X-Secret, X-Nubo-Timestamp")

		// 4. Quels headers le client a le droit de LIRE dans la réponse ?
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, X-Signature, X-Timestamp")

		// 5. Gestion des Cookies / Credentials
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		// 6. INTERCEPTION DE LA MÉTHODE OPTIONS
		if c.Request.Method == "OPTIONS" {
			// Si c'est un OPTIONS, on dit "C'est bon, circulez" et on arrête là.
			// On renvoie 204 (No Content)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		// Si c'est une vraie requête (GET, POST...), on passe à la suite
		c.Next()
	}
}
