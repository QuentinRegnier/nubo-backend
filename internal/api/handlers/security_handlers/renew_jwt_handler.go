package security_handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/security_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// RenewJWT godoc
// @Summary      Renouveler le JWT (Ratchet Rotation)
// @Description  Génère un nouveau JWT pour l'utilisateur et effectue une rotation de sécurité des secrets (Ratchet).
// @Description  Cette route est critique et nécessite une signature HMAC valide basée sur le secret actuel de la session.
// @Description
// @Description  **Mécanisme :**
// @Description  1. Vérifie la signature HMAC du body avec les headers de sécurité.
// @Description  2. Identifie la session via l'ID utilisateur et le `X-Secret`.
// @Description  3. Calcule le prochain secret (N+1) et met à jour l'historique (Ratchet).
// @Description  4. Renvoie le nouveau JWT.
// @Description
// @Description  **Règles & Erreurs :**
// @Description
// @Description  🔴 **400 Bad Request :**
// @Description  * `Erreur lecture body` : Impossible de lire le corps de la requête.
// @Description  * `Invalid JSON format` : Le JSON envoyé est mal formé.
// @Description  * `Headers de sécurité manquants` : Il manque `Authorization`, `X-Secret`, `X-Signature` ou `X-Timestamp`.
// @Description
// @Description  🟠 **401 Unauthorized :**
// @Description  * `Signature HMAC invalide` : La signature ne correspond pas au contenu (tentative de falsification).
// @Description  * `Session invalide ou Secret incorrect` : Le secret fourni ne correspond à aucune session active pour cet utilisateur (ou désynchronisation Ratchet).
// @Description
// @Description  ⚫ **500 Internal Server Error :**
// @Description  * `Erreur génération token` : Échec de la création du JWT.
// @Description  * `Erreur rotation secrets` : Impossible de mettre à jour Redis (Ratchet bloqué).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <Last_JWT>"
// @Param        X-Secret      header string true "Secret actuel de la session"
// @Param        X-Signature   header string true "Signature HMAC calculée"
// @Param        X-Timestamp   header string true "Timestamp de la requête"
// @Success      200  {object}  domain.RenewJWTResponse
// @Failure      400  {object}  domain.ErrorResponse "Requête invalide"
// @Failure      401  {object}  domain.ErrorResponse "Authentification / Signature refusée"
// @Failure      500  {object}  domain.ErrorResponse "Erreur serveur critique"
// @Router       /renew-jwt [post_service]
func RenewJWT(c *gin.Context) {
	// 1. Lire le Body uniquement pour la signature HMAC (Raw bytes)
	// On ne fait plus de json.Unmarshal ici car on récupère les infos du Token
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Erreur lecture body"})
		return
	}
	// On n'a pas besoin de restaurer le body avec NopCloser car on ne le relit plus après

	// 2. Récupération des Headers
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		authHeader = authHeader[7:]
	}
	clientSecret := c.GetHeader("X-Secret")
	clientHMAC := c.GetHeader("X-Signature")
	clientTs := c.GetHeader("X-Timestamp")

	if authHeader == "" || clientSecret == "" || clientHMAC == "" || clientTs == "" {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Headers de sécurité manquants"})
		return
	}

	// 3. EXTRACTION DES DONNÉES DU JWT (Même périmé)
	// On utilise ParseUnverified de la lib jwt/v5
	token, _, err := new(jwt.Parser).ParseUnverified(authHeader, jwt.MapClaims{})
	if err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Token illisible"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Claims JWT invalides"})
		return
	}

	// A. Récupération UserID ("sub")
	sub, err := claims.GetSubject()
	if err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "UserID manquant dans le token"})
		return
	}
	userID, err := strconv.ParseInt(sub, 10, 64) // Conversion en int64
	if err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format UserID invalide"})
		return
	}

	// B. Récupération DeviceToken ("dev")
	deviceToken, ok := claims["dev"].(string)
	if !ok || deviceToken == "" {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "DeviceToken manquant dans le token"})
		return
	}

	// 4. Vérification HMAC
	// On signe toujours avec le bodyBytes (même vide) pour garantir l'intégrité de la requête
	contentToSign := security.GetBodyToSign(c.Request, bodyBytes)
	stringToSign := security.BuildStringToSign(c.Request.Method, c.Request.URL.Path, clientTs, contentToSign)

	if !security.CheckHMAC(stringToSign, clientSecret, clientHMAC) {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Signature HMAC invalide"})
		return
	}

	// 5. Génération Nouveau JWT (Action Serveur)
	// IMPORTANT : On remet le deviceToken dans le nouveau JWT pour la suite !
	newJWT, err := pkg.GenerateToken(userID, deviceToken, variables.JWTExpirationSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Erreur génération token"})
		return
	}

	// 6. Rotation du Ratchet & Mise à jour Session
	// On utilise le userID extrait du token et le authHeader comme "LastJWT"
	if err := security.RotateRatchet(c, userID, clientSecret, authHeader); err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Session invalide ou Secret incorrect"})
		return
	}

	// 7. PRÉPARATION DE LA RÉPONSE SIGNÉE
	// On prépare l'objet réponse
	respData := security_models.RenewJWTResponse{
		Token:   newJWT,
		Message: "Renouvellement OK",
	}

	// A. Sérialisation manuelle en JSON pour la signature
	respBytes, err := json.Marshal(respData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Erreur encoding réponse"})
		return
	}

	// B. Génération du Timestamp et Signature
	respTs := fmt.Sprintf("%d", time.Now().Unix())

	// C. Build StringToSign (Response Binding)
	// On signe : METHOD | PATH | TS_REPONSE | BODY_REPONSE
	stringToSignResp := security.BuildStringToSign(
		c.Request.Method,
		c.Request.URL.Path,
		respTs,
		string(respBytes),
	)

	// D. Calcul HMAC
	// Règle : On utilise le secret qui a validé la requête (clientSecret)
	// C'est ce secret qui est devenu 'LastSecret' dans la BDD après la rotation.
	h := hmac.New(sha256.New, []byte(clientSecret))
	h.Write([]byte(stringToSignResp))
	respSig := hex.EncodeToString(h.Sum(nil))

	// E. Ajout des Headers
	c.Header("X-Timestamp", respTs)
	c.Header("X-Signature", respSig)

	// F. Envoi de la réponse
	c.Data(http.StatusOK, "application/json", respBytes)
}
