package security_handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/security_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/security"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
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
// @Description  * Reset du Ratchet (Secret 0 = NewMaster, Secret 1 = FirebaseInstallationID).
// @Description  * Mise à jour asynchrone de Postgres et Mongo pour persister le nouveau MasterToken.
// @Description
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        Authorization header string false "Bearer <Current_JWT> (Optionnel, pour continuité)"
// @Param        X-Signature   header string true  "HMAC calculé avec l'ANCIEN MasterToken"
// @Param        X-Timestamp   header string true  "Timestamp de la requête"
// @Param        input         body   security_models.RefreshMasterInput true "Données de reset (MasterToken, UserID, Username)"
// @Success      200  {object}  domain.RefreshMasterResponse "Nouveaux identifiants générés"
// @Failure      400  {object}  nubo_error.PublicErrorResponse "Format invalide ou Headers manquants"
// @Failure      401  {object}  nubo_error.PublicErrorResponse "MasterToken introuvable ou Signature HMAC invalide"
// @Failure      500  {object}  nubo_error.PublicErrorResponse "Erreur serveur critique (Génération/Sauvegarde)"
// @Router       /auth/refresh-master [post]
func RefreshMaster(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("READ_BODY_ERROR", "Erreur de lecture du body.", err))
		return
	}

	var input security_models.RefreshMasterInput
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &input); err != nil {
			nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide.", err))
			return
		}
	} else {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("EMPTY_BODY", "Le corps de la requête est requis.", nil))
		return
	}

	if err := pkg.ValidateStruct(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("VALIDATION_FAILED", "Validation failed.", err))
		return
	}

	authHeader := c.GetHeader("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		authHeader = authHeader[7:]
	} else {
		authHeader = ""
	}

	clientHMAC := c.GetHeader("X-Signature")
	clientTs := c.GetHeader("X-Timestamp")

	if clientHMAC == "" || clientTs == "" {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("MISSING_HEADERS", "Headers de sécurité manquants.", nil))
		return
	}

	var sessionRaw auth_models.SessionsPayload
	var sessionFound bool

	if s, err := cache_service.LoadSessionFromCache(c, input.UserID, "", input.MasterToken); err == nil && s.ID != 0 {
		sessionRaw = s
		sessionFound = true
	}

	if !sessionFound {
		if s, err := mongo.MongoLoadSession(input.UserID, "", input.MasterToken, ""); err == nil && s.ID != 0 {
			sessionRaw = s
			sessionFound = true
			_ = cache_service.SetSessionInCache(c, sessionRaw)
		}
	}

	if !sessionFound {
		s, err := postgres.FuncLoadSession(-1, input.UserID, "", input.MasterToken)
		if err == nil && s.ID != 0 {
			sessionRaw = s
			sessionFound = true
			_ = redis.EnqueueDB(c, s.ID, 0, redis.EntitySession, redis.ActionCreate, s, redis.TargetMongo)
			_ = cache_service.SetSessionInCache(c, s)
		}
	}

	if !sessionFound || sessionRaw.ID == 0 {
		nubo_error.RespondWithError(c, nubo_error.NewForbidden("SESSION_NOT_FOUND", "Session introuvable.", nil))
		return
	}

	contentToSign := security.GetBodyToSign(c.Request, bodyBytes)
	stringToSign := security.BuildStringToSign(c.Request.Method, c.Request.URL.Path, clientTs, contentToSign)

	if !security.CheckHMAC(stringToSign, input.MasterToken, clientHMAC) {
		nubo_error.RespondWithError(c, nubo_error.NewForbidden("INVALID_HMAC", "Signature HMAC invalide (Master Check).", nil))
		return
	}

	newMasterToken, err := pkg.GenerateToken(input.UserID, sessionRaw.FirebaseInstallationID, variables.MasterTokenExpirationSeconds)
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewInternal(err))
		return
	}

	newJWT, err := pkg.GenerateToken(input.UserID, sessionRaw.FirebaseInstallationID, variables.JWTExpirationSeconds)
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewInternal(err))
		return
	}

	if sessionRaw.CurrentSecret, err = security.ResetRatchet(newMasterToken, sessionRaw.FirebaseInstallationID); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewInternal(err))
		return
	}

	sessionRaw.MasterToken = newMasterToken
	sessionRaw.LastSecret = sessionRaw.FirebaseInstallationID
	sessionRaw.LastJWT = authHeader
	sessionRaw.ToleranceTime = time.Now().Add(time.Duration(variables.ToleranceTimeSeconds) * time.Second)
	sessionRaw.ExpiresAt = time.Now().Add(time.Duration(variables.MasterTokenExpirationSeconds) * time.Second)

	if errAdd := cache_service.SetSessionInCache(c, sessionRaw); errAdd != nil {
		logger.Log.Warn().Err(errAdd).Msg("Warning: Echec update Session Cache L1")
	}

	if err := redis.EnqueueDB(c, sessionRaw.ID, 0, redis.EntitySession, redis.ActionUpdate, sessionRaw, redis.TargetAll); err != nil {
		logger.Log.Error().Err(err).Msg("Error enqueuing to DB")
	}

	respData := security_models.RefreshMasterResponse{
		MasterToken: newMasterToken,
		Token:       newJWT,
		Message:     "Master Reset Successful",
	}

	respBytes, err := json.Marshal(respData)
	if err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewInternal(err))
		return
	}

	respTs := fmt.Sprintf("%d", time.Now().Unix())

	stringToSignResp := security.BuildStringToSign(
		c.Request.Method,
		c.Request.URL.Path,
		respTs,
		string(respBytes),
	)

	h := hmac.New(sha256.New, []byte(input.MasterToken))
	h.Write([]byte(stringToSignResp))
	respSig := hex.EncodeToString(h.Sum(nil))

	c.Header("X-Timestamp", respTs)
	c.Header("X-Signature", respSig)

	c.Data(http.StatusOK, "application/json", respBytes)
}
