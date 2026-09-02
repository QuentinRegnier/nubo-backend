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
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Requête invalide"
// @Failure      401  {object}  nubo_error.PublicErrorResponse "Authentification / Signature refusée"
// @Failure      500  {object}  nubo_error.PublicErrorResponse "Erreur serveur critique"
// @Router       /renew-jwt [post_service]
func RenewJWT(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("READ_BODY_ERROR", "Erreur lecture body.", err))
		return
	}

	// 2. Récupération des Headers
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		authHeader = authHeader[7:]
	}
	clientSecret := c.GetHeader("X-Secret")
	clientHMAC := c.GetHeader("X-Signature")
	clientTs := c.GetHeader("X-Timestamp")

	if authHeader == "" || clientSecret == "" || clientHMAC == "" || clientTs == "" {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("MISSING_HEADERS", "Headers de sécurité manquants.", nil))
		return
	}

	// 3. EXTRACTION DES DONNÉES DU JWT
	token, _, err := new(jwt.Parser).ParseUnverified(authHeader, jwt.MapClaims{})
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_JWT", "Token illisible.", err))
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_CLAIMS", "Claims JWT invalides.", nil))
		return
	}

	sub, err := claims.GetSubject()
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("MISSING_SUBJECT", "UserID manquant dans le token.", err))
		return
	}
	userID, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_SUBJECT", "Format UserID invalide.", err))
		return
	}

	firebaseInstallationID, ok := claims["dev"].(string)
	if !ok || firebaseInstallationID == "" {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("MISSING_DEVICE_ID", "FirebaseInstallationID manquant dans le token.", nil))
		return
	}

	// 4. Vérification HMAC
	contentToSign := security.GetBodyToSign(c.Request, bodyBytes)
	stringToSign := security.BuildStringToSign(c.Request.Method, c.Request.URL.Path, clientTs, contentToSign)

	if !security.CheckHMAC(stringToSign, clientSecret, clientHMAC) {
		nubo_error.RespondWithError(c, nubo_error.NewForbidden("INVALID_HMAC", "Signature HMAC invalide.", nil))
		return
	}

	// 5. Génération Nouveau JWT
	newJWT, err := pkg.GenerateToken(userID, firebaseInstallationID, variables.JWTExpirationSeconds)
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewInternal(err))
		return
	}

	// 6. Rotation du Ratchet & Mise à jour Session
	if err := security.RotateRatchet(c, userID, firebaseInstallationID, clientSecret, authHeader); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 7. PRÉPARATION DE LA RÉPONSE SIGNÉE
	respData := security_models.RenewJWTResponse{
		Token:   newJWT,
		Message: "Renouvellement OK",
	}

	respBytes, err := json.Marshal(respData)
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewInternal(err))
		return
	}

	respTs := fmt.Sprintf("%d", time.Now().Unix())
	stringToSignResp := security.BuildStringToSign(c.Request.Method, c.Request.URL.Path, respTs, string(respBytes))

	h := hmac.New(sha256.New, []byte(clientSecret))
	h.Write([]byte(stringToSignResp))
	respSig := hex.EncodeToString(h.Sum(nil))

	c.Header("X-Timestamp", respTs)
	c.Header("X-Signature", respSig)
	c.Data(http.StatusOK, "application/json", respBytes)
}
