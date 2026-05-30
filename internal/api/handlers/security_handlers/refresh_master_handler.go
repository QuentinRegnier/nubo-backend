package security_handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/security_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/gin-gonic/gin"
)

// RefreshMaster godoc
// @Summary      Hard Refresh (Master Token Rotation)
// @Description  Réinitialise toute la chaîne de sécurité (Ratchet, JWT, Secrets) en générant un nouveau MasterToken.
// @Description  Cette route est l'ultime recours ("Last Resort") lorsque le Ratchet est désynchronisé ou que le JWT est expiré depuis trop longtemps.
// @Description
// @Description  **Mécanisme de Résilience :**
// @Description  1. Recherche la session via le MasterToken dans **Redis**.
// @Description  2. Si introuvable (crash cache_service), cherche dans **MongoDB**.
// @Description  3. Si introuvable, cherche dans **PostgreSQL** (Source de vérité).
// @Description  4. Si trouvé, valide la signature HMAC et réinitialise tout.
// @Description
// @Description  **Actions Serveur :**
// @Description  * Génération de `NewMasterToken` et `NewJWT`.
// @Description  * Reset du Ratchet (Secret 0 = NewMaster, Secret 1 = DeviceToken).
// @Description  * Mise à jour asynchrone de Postgres et Mongo pour persister le nouveau MasterToken.
// @Description
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        Authorization header string false "Bearer <Current_JWT> (Optionnel, pour continuité)"
// @Param        X-Signature   header string true  "HMAC calculé avec l'ANCIEN MasterToken"
// @Param        X-Timestamp   header string true  "Timestamp de la requête"
// @Param        request       body     domain.RefreshMasterInput true "Données de reset (MasterToken, UserID, Username)"
// @Success      200  {object}  domain.RefreshMasterResponse "Nouveaux identifiants générés"
// @Failure      400  {object}  domain.ErrorResponse "Format invalide ou Headers manquants"
// @Failure      401  {object}  domain.ErrorResponse "MasterToken introuvable ou Signature HMAC invalide"
// @Failure      500  {object}  domain.ErrorResponse "Erreur serveur critique (Génération/Sauvegarde)"
// @Router       /auth/refresh-master [post_service]
func RefreshMaster(c *gin.Context) {
	// 1. Lecture du Body (Nécessaire pour le calcul HMAC manuel plus bas)
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Erreur lecture body"})
		return
	}

	// [IMPORTANT] Restaurer le body pour que c.PostForm puisse le lire ensuite
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 2. Parsing des données (Support Form-Data ET Raw JSON)
	var input security_models.RefreshMasterInput
	jsonData := c.PostForm("data")

	if jsonData != "" {
		// CAS 1 : Multipart/Form-Data (celui que tu veux utiliser)
		if err := json.Unmarshal([]byte(jsonData), &input); err != nil {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format in 'data': " + err.Error()})
			return
		}
	} else {
		// CAS 2 : Raw JSON (Fallback, au cas où)
		// Si 'data' est vide, on essaie de parser le body entier
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &input); err != nil {
				c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Invalid JSON format"})
				return
			}
		} else {
			// Aucun contenu trouvé
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "The 'data' field containing the JSON is required"})
			return
		}
	}

	// 2. Headers
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		authHeader = authHeader[7:]
	} else {
		authHeader = "" // Cas NULL accepté (perte du JWT)
	}

	clientHMAC := c.GetHeader("X-Signature")
	clientTs := c.GetHeader("X-Timestamp")

	if clientHMAC == "" || clientTs == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{Error: "Headers de sécurité manquants"})
		return
	}

	// 5. Récupération de la Session (Cascade : Redis -> Mongo -> Postgres)
	var sessionRaw models.SessionsRequest
	var sessionFound bool

	// A. Essai Redis
	if s, err := redis.RedisLoadSession(input.UserID, "", input.MasterToken, ""); err == nil && s.ID != 0 {
		sessionRaw = s
		sessionFound = true
	}

	// B. Essai Mongo
	if !sessionFound {
		if s, err := mongo.MongoLoadSession(input.UserID, "", input.MasterToken, ""); err == nil && s.ID != 0 {
			sessionRaw = s
			sessionFound = true
			_ = redis.RedisCreateSession(sessionRaw) // Repopulation cache_service
		}
	}

	// C. Essai Postgres
	if !sessionFound {
		s, err := postgres.FuncLoadSession(-1, input.UserID, "", input.MasterToken)
		if err == nil && s.ID != 0 {
			sessionRaw = s
			sessionFound = true
			// Repopulation des backups
			_ = redis.EnqueueDB(c, s.ID, 0, redis.EntitySession, redis.ActionCreate, s, redis.TargetMongo)
			_ = redis.RedisCreateSession(s)
		}
	}

	// SÉCURITÉ CRITIQUE : Si après les 3 essais on n'a rien, on arrête TOUT.
	if !sessionFound || sessionRaw.ID == 0 {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Session introuvable"})
		return
	}

	// 4. Vérification HMAC (Master Check)
	contentToSign := security.GetBodyToSign(c.Request, bodyBytes)
	fmt.Printf("ContentToSign for Master Check: %s\n", contentToSign)                                                     // --- IGNORE ---
	fmt.Printf("Arguments for Master Check: Method=%s, Path=%s, Ts=%s\n", c.Request.Method, c.Request.URL.Path, clientTs) // --- IGNORE ---
	stringToSign := security.BuildStringToSign(c.Request.Method, c.Request.URL.Path, clientTs, contentToSign)
	fmt.Printf("StringToSign for Master Check: %s\n", stringToSign) // --- IGNORE ---
	if !security.CheckHMAC(stringToSign, input.MasterToken, clientHMAC) {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{Error: "Signature HMAC invalide (Master Check)"})
		return
	}

	// 6. Génération des Nouveaux Credentials
	newMasterToken, err := pkg.GenerateToken(input.UserID, sessionRaw.DeviceToken, variables.MasterTokenExpirationSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur génération MasterToken"})
		return
	}

	newJWT, err := pkg.GenerateToken(input.UserID, sessionRaw.DeviceToken, variables.JWTExpirationSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur génération JWT"})
		return
	}

	// 7. Reset du Ratchet dans Redis
	if sessionRaw.CurrentSecret, err = security.ResetRatchet(c, sessionRaw.ID, newMasterToken, sessionRaw.DeviceToken, authHeader); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur reset Ratchet"})
		return
	}

	// 8. Mise à jour des Bases de Données (Redis Sync + Queue Async)

	// Mise à jour de l'objet local
	sessionRaw.MasterToken = newMasterToken
	sessionRaw.LastSecret = sessionRaw.DeviceToken
	sessionRaw.LastJWT = authHeader
	sessionRaw.ToleranceTime = time.Now().Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second)
	sessionRaw.ExpiresAt = time.Now().Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)

	// A. Redis (Immédiat pour le cache_service)
	if errAdd := redis.RedisUpdateSession(sessionRaw); errAdd != nil {
		fmt.Printf("⚠️ Warning: Echec update Redis: %v\n", errAdd)
	}

	// TargetBoth : On veut mettre à jour Mongo (Doc) ET Postgres (Relationnel)
	// car le MasterToken a changé (info critique).
	if err := redis.EnqueueDB(c, sessionRaw.ID, 0, redis.EntitySession, redis.ActionUpdate, sessionRaw, redis.TargetAll); err != nil {
		log.Printf("Error enqueuing to DB: %v", err)
	}

	// 9. PRÉPARATION DE LA RÉPONSE SIGNÉE
	respData := security_models.RefreshMasterResponse{
		MasterToken: newMasterToken,
		Token:       newJWT,
		Message:     "Master Reset Successful",
	}

	// A. Sérialisation
	respBytes, err := json.Marshal(respData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{Error: "Erreur encoding réponse"})
		return
	}

	// B. Timestamp
	respTs := fmt.Sprintf("%d", time.Now().Unix())

	// C. StringToSign
	stringToSignResp := security.BuildStringToSign(
		c.Request.Method,
		c.Request.URL.Path,
		respTs,
		string(respBytes),
	)

	// D. Calcul HMAC
	// Règle : On utilise l'ANCIEN MasterToken (input.MasterToken) car le client ne connait pas encore le nouveau.
	h := hmac.New(sha256.New, []byte(input.MasterToken))
	h.Write([]byte(stringToSignResp))
	respSig := hex.EncodeToString(h.Sum(nil))

	// E. Headers
	c.Header("X-Timestamp", respTs)
	c.Header("X-Signature", respSig)

	// F. Envoi
	c.Data(http.StatusOK, "application/json", respBytes)
}
