package notification_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/notification_service"
	"github.com/gin-gonic/gin"
)

// ReadNotificationsHandler godoc
// @Summary      Marquer des notifications comme lues
// @Description  Met à jour le curseur de lecture (watermark) de l'utilisateur et diffuse cet état en temps réel sur tous ses appareils connectés via WebSockets.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée nécessitant un token JWT d'authentification valide.
// @Description  - Required permissions or roles: L'utilisateur modifie exclusivement son propre curseur de lecture.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. **Validation et Identification :**
// @Description     - Parsing du payload JSON (`ReadUpToID`).
// @Description     - Extraction de l'ID utilisateur depuis le contexte HTTP.
// @Description  2. **Opération O(1) en RAM :**
// @Description     - Appel à `notification_service.MarkNotificationsAsRead`.
// @Description     - Écriture instantanée du nouveau curseur (`ReadUpToID`) directement dans Redis (`NotificationCursors.SetPrimitive`).
// @Description  3. **Synchronisation et Effets de Bord :**
// @Description     - Distribution d'un événement de mise à jour de lecture (`NotificationRead`) via WebSockets (`realtime_service.DistributeToUsers`) pour synchronisation des autres appareils.
// @Description     - Marquage de l'activité utilisateur via `cache_service.TouchActivityTimestamp` (Dirty Flag d'activité globale).
// @Description  4. **Réponse :**
// @Description     - Retour au client du timestamp de mise à jour sans attendre le résultat de la distribution WebSocket.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[numan_error.CodeInvalidPayload] Requête mal formatée ou paramètres invalides:**
// @Description    - Trigger: Format JSON invalide ou `ReadUpToID` manquant.
// @Description    - Execution stage: Validation GIN.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInvalidPayload`
// @Description
// @Description  🟡 **401 Unauthorized:**
// @Description
// @Description  - **[numan_error.CodeUnauthorized] Jeton invalide ou absent:**
// @Description    - Trigger: Le client HTTP n'envoie pas de token, ou le JWT a expiré.
// @Description    - Execution stage: Middleware global d'authentification.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeUnauthorized`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[numan_error.CodeInternalError] Erreur de service inattendue:**
// @Description    - Trigger: Inaccessibilité critique du cluster Redis empêchant l'écriture du curseur.
// @Description    - Execution stage: Exécution profonde du service métier.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError`
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   notification_models.ReadNotificationsInput true "Payload pour marquer les notifications comme lues jusqu'à un certain ID"
// @Success      200  {object}  notification_models.ReadNotificationsOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /notifications/read [post]
func ReadNotificationsHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input notification_models.ReadNotificationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := notification_service.MarkNotificationsAsRead(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
