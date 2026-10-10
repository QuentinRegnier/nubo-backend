package auth_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// DeleteSessionHandler godoc
// @Summary      Révoquer une session à distance
// @Description  Permet à un utilisateur de détruire spécifiquement l'une de ses autres sessions actives (par exemple pour déconnecter un autre appareil).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée. Requiert un JWT valide et une signature HMAC.
// @Description  - Required permissions or roles: Le `userID` de la requête (extrait du contexte) doit être strictement identique au `UserID` propriétaire de la session cible.
// @Description  - Relevant middleware: JWTMiddleware, HMACMiddleware, MaxBodySize, RateLimiter, CORS, Recovery.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `session_id` (entier 64-bit) transmis dans le corps JSON.
// @Description  - Validation rules: Binding GIN standard sur `auth_models.DeleteSessionInput`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Extraction du `callerID` depuis le contexte sécurisé de la requête.
// @Description  2. Validation du payload JSON pour récupérer le `session_id` cible.
// @Description  3. Recherche de la session visée (Tentative L1 RAM -> Fallback L3 PostgreSQL).
// @Description  4. Contrôle strict de sécurité : vérification que la session appartient bien à l'utilisateur courant.
// @Description  5. Suppression immédiate de la session (Objet et Index) dans le cache L1.
// @Description  6. Déclenchement d'un événement temps réel WebSocket (`NotificationSessionRevoked`) pour déconnecter l'appareil cible en direct.
// @Description  7. File d'attente asynchrone (Write-Behind vers Redis) ordonnant un Hard Delete en base de données.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: JSON `{"message": "Session révoquée avec succès"}`.
// @Description  - Persistence guarantees: L'invalidation en RAM est synchrone. L'effacement physique en BDD est asynchrone (Write-Behind).
// @Description  - Side effects: Purge L1, Websocket notifié, worker Redis mobilisé pour destruction.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[INVALID_PAYLOAD] Invalid JSON or missing session_id:**
// @Description    - Trigger: Le champ `session_id` est absent ou d'un type invalide.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⛔ **403 Forbidden:**
// @Description
// @Description  - **[FORBIDDEN] Ownership mismatch:**
// @Description    - Trigger: L'utilisateur tente de supprimer une session qui appartient à un autre compte.
// @Description    - Execution stage: Contrôle de propriété métier.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **404 Not Found:**
// @Description
// @Description  - **[NOT_FOUND] Session does not exist:**
// @Description    - Trigger: Le `session_id` est introuvable en RAM (L1) ET en base de données (L3), suggérant une révocation antérieure ou une ID inexistante.
// @Description    - Execution stage: Recherche L1/L3 en cascade.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[INTERNAL_ERROR] Database or Queue failure:**
// @Description    - Trigger: Échec de l'interrogation PostgreSQL (L3) ou échec critique lors de l'insertion dans la file d'attente Redis (risque d'état fantôme).
// @Description    - Execution stage: Chargement L3 / Mise en file d'attente.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         sessions
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   auth_models.DeleteSessionInput true "Identifiant de la session à révoquer"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden (Not owner)"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Session Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /session/delete [delete]
func DeleteSessionHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input auth_models.DeleteSessionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou session_id manquant.", err))
		return
	}

	if err := auth_service.RevokeSession(c.Request.Context(), callerID, input.SessionID); err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session révoquée avec succès"})
}
