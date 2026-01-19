package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/gin-gonic/gin"
)

// -------------------------------------------------------------------------
// WRAPPER POUR INTERCEPTER LA RÉPONSE
// -------------------------------------------------------------------------

// responseBodyWriter permet de capturer le corps de la réponse pour le signer
// avant qu'il ne soit envoyé au client.
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write capture les données dans le buffer au lieu de les envoyer direct
func (w responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b) // On stocke en mémoire
	return len(b), nil
}

// -------------------------------------------------------------------------
// MIDDLEWARE
// -------------------------------------------------------------------------

func HMACMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// =====================================================================
		// PARTIE 1 : VÉRIFICATION DE LA REQUÊTE (ENTRANTE)
		// =====================================================================

		// 1. Headers Client
		clientTs := c.GetHeader("X-Timestamp")
		clientSig := c.GetHeader("X-Signature")

		if clientTs == "" || clientSig == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Headers de sécurité manquants"})
			return
		}

		// 2. Contexte (placé par JWT Middleware)
		userIDRaw, existsUID := c.Get("userID")
		deviceTokenRaw, existsDev := c.Get("deviceToken")

		if !existsUID || !existsDev {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Contexte d'authentification manquant"})
			return
		}

		userIDStr := fmt.Sprintf("%v", userIDRaw)
		userID, _ := strconv.Atoi(userIDStr)
		deviceToken := fmt.Sprintf("%v", deviceTokenRaw)

		// 3. Récupération Session Redis
		//filter := map[string]any{
		//	"user_id":      map[string]any{"$eq": userID},
		//	"device_token": map[string]any{"$eq": deviceToken},
		//}

		//sessionsData, err := redis.Sessions.Get(c, filter)
		//if err != nil || len(sessionsData) == 0 {
		//	c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Session invalide ou expirée"})
		//	return
		//}

		var session domain.SessionsRequest
		var sessionFound bool = false

		// A. Essai Redis
		session, err := redis.RedisLoadSession(userID, deviceToken, "", "")
		if err == nil && session.ID != 0 {
			sessionFound = true
		}

		if !sessionFound {
			// Supposons que tu aies un repo mongo générique similaire
			session, errMongo := mongo.MongoLoadSession(userID, deviceToken, "", "")
			if errMongo == nil && session.ID != 0 {
				sessionFound = true
				if errAdd := redis.RedisCreateSession(session); errAdd != nil {
					fmt.Printf("⚠️ Warning: Echec repopulation Redis depuis MongoDB: %v\n", errAdd)
				}
			}
		}

		if !sessionFound {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Session invalide ou expirée"})
			return
		}

		// 4. Anti-Rejeu (Timestamp)
		tsInt, err := strconv.ParseInt(clientTs, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Timestamp invalide"})
			return
		}
		now := time.Now().Unix()
		if math.Abs(float64(now-tsInt)) > variables.ToleranceTimeSeconds {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Requête expirée"})
			return
		}

		// 5. Lecture et Validation HMAC Requête
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		stringToSignReq := security.BuildStringToSign(c.Request.Method, c.Request.URL.Path, clientTs, string(bodyBytes))

		// On initialise avec le secret actuel par défaut
		usedSecret := session.CurrentSecret

		// 1. Essai avec le secret actuel (Cas nominal)
		isValid := security.CheckHMAC(stringToSignReq, session.CurrentSecret, clientSig)

		// 2. Si échec, essai avec l'ancien secret (Cas tolérance)
		if !isValid && session.LastSecret != "" {
			// CONDITION STRICTE : Uniquement si on est encore dans la fenêtre de tolérance
			if !session.ToleranceTime.IsZero() && time.Now().Before(session.ToleranceTime) {
				if security.CheckHMAC(stringToSignReq, session.LastSecret, clientSig) {
					isValid = true
					usedSecret = session.LastSecret // On note qu'on utilise l'ancien secret
				}
			}
		}

		if !isValid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Signature HMAC invalide"})
			return
		}

		// =====================================================================
		// PARTIE 2 : INTERCEPTION ET SIGNATURE DE LA RÉPONSE (SORTANTE)
		// =====================================================================

		// On remplace le Writer par défaut de Gin par notre captureur
		w := &responseBodyWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = w

		// EXÉCUTION DU HANDLER (CreatePost, GetPosts, etc.)
		// Le handler va faire c.JSON(...), mais cela va écrire dans w.body au lieu du réseau
		c.Next()

		// --- Une fois le handler terminé ---

		// 1. Récupérer le contenu généré par le handler
		responseBody := w.body.Bytes()

		// 2. Générer le Timestamp de réponse
		respTs := fmt.Sprintf("%d", time.Now().Unix())

		// 3. Construire la chaîne à signer pour la RÉPONSE
		// Convention Uniforme : METHOD|PATH|TIMESTAMP|BODY
		// On lie la réponse à la requête initiale (Response Binding) pour plus de sécurité.
		stringToSignResp := security.BuildStringToSign(
			c.Request.Method,
			c.Request.URL.Path,
			respTs,
			string(responseBody),
		)

		// 4. Signer avec le Secret utilisé pour la requête (Cohérence)
		// Si le client a utilisé LastSecret, on signe avec LastSecret.
		h := hmac.New(sha256.New, []byte(usedSecret))
		h.Write([]byte(stringToSignResp))
		respSig := hex.EncodeToString(h.Sum(nil))

		// 5. Ajouter les Headers AVANT d'envoyer le corps
		w.ResponseWriter.Header().Set("X-Timestamp", respTs)
		w.ResponseWriter.Header().Set("X-Signature", respSig)

		// 6. Envoyer réellement les données au client
		// Attention : on utilise w.ResponseWriter qui est l'original
		w.ResponseWriter.Write(responseBody)
	}
}
