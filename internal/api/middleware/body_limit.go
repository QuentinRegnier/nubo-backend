package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// Limite pour le texte brut / JSON (2 Mégaoctets)
	MaxJSONSize = 2 << 20

	// Limite pour les envois de médias / Multipart (15 Mégaoctets)
	MaxMultipartSize = 15 << 20
)

// MaxBodySize est la guillotine qui protège la RAM et le CPU
func MaxBodySize() gin.HandlerFunc {
	return func(c *gin.Context) {
		contentType := c.Request.Header.Get("Content-Type")

		var limit int64

		// Définir la limite selon le type de requête
		if strings.HasPrefix(contentType, "multipart/form-data") {
			limit = MaxMultipartSize
		} else {
			limit = MaxJSONSize
		}

		// http.MaxBytesReader bloque automatiquement la lecture si le body dépasse "limit".
		// Ce n'est pas juste un check de header, ça coupe littéralement le flux réseau TCP.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)

		// On tente de forcer la lecture d'un premier octet pour déclencher l'erreur immédiatement
		// si le Content-Length envoyé par le client dépasse déjà la limite.
		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "Payload too large. Maximum allowed size exceeded.",
			})
			return
		}

		c.Next()
	}
}
