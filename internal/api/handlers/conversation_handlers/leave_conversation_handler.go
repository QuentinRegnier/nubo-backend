package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// LeaveConversationHandler godoc
// @Summary      Quitter une conversation
// @Description  Permet à un utilisateur de se retirer volontairement d'une conversation.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description  - Security checks: Si l'utilisateur est l'unique propriétaire (Owner) d'un groupe, la politique l'oblige à désigner un nouveau propriétaire avant de pouvoir s'échapper.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `conversation_id`.
// @Description  - Optional fields: `new_owner_id` (requis si l'utilisateur est propriétaire).
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Binding des données de requête.
// @Description  2. Vérification d'appartenance et du rôle du requérant.
// @Description  3. Contrôle métier : Si le requérant est propriétaire (`IsOwner`), la requête est rejetée (403) si `new_owner_id` n'est pas fourni. Le nouveau propriétaire désigné doit être un administrateur actif.
// @Description  4. Application métier (Soft Delete / Suppression Totale du Cache). L'entrée est purgée du ZSET de l'inbox.
// @Description  5. La demande de suppression du membre est ajoutée en file (Write-Behind L2/L3).
// @Description  6. Mise à jour de l'activité. Un broadcast websocket informe le reste du groupe.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.LeaveConversationOutput`.
// @Description  - Persistence guarantees: Cache L1 synchrone, file Redis Write-Behind pour suppression base de données.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Paramètres invalides:**
// @Description    - **Trigger:** Le format JSON est invalide ou le `new_owner_id` désigné n'est pas qualifié.
// @Description    - **Execution stage:** Phase de binding GIN et validation initiale des entrées, avant l'exécution de la logique métier.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeInvalidPayload` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🟠 **401 Unauthorized:**
// @Description  - **[UNAUTHORIZED]**
// @Description    - **Trigger:** Le token JWT est absent de l'en-tête Authorization, mal formaté, expiré ou invalide.
// @Description    - **Execution stage:** Middleware d'authentification JWT, avant que la requête n'atteigne le handler.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeUnauthorized` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  🔴 **403 Forbidden:**
// @Description  - **[FORBIDDEN_ACCESS] Transfert de propriété requis:**
// @Description    - **Trigger:** L'utilisateur tente de fuir sa responsabilité de propriétaire sans désigner de successeur valide.
// @Description    - **Execution stage:** Évaluation des permissions métier dans le handler ou le service appelé, après authentification.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **404 Not Found:**
// @Description  - **[RESOURCE_NOT_FOUND] Détails introuvables:**
// @Description    - **Trigger:** Impossible de charger la conversation (elle n'existe pas ou le cache est rompu).
// @Description    - **Execution stage:** Tentative de chargement de la ressource dans le cache L1 ou la base de données L2/L3.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeNotFound` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description  - **[INTERNAL_SERVER_ERROR]**
// @Description    - **Trigger:** Une erreur inattendue est survenue lors du traitement, par exemple une perte de connexion avec la base de données ou le cache Redis.
// @Description    - **Execution stage:** À n'importe quel point du traitement interne, généralement lors d'un appel à un service externe ou une base de données.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   conversation_models.LeaveConversationInput true "Payload"
// @Success      200  {object}  conversation_models.LeaveConversationOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error"
// @Router       /conversation/delete [delete]
func LeaveConversationHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input conversation_models.LeaveConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou conversation_id manquant.", err))
		return
	}

	output, errService := conversation_service.LeaveConversation(c.Request.Context(), callerID, input)
	if errService != nil {
		numan_error.RespondWithError(c, errService)
		return
	}

	c.JSON(http.StatusOK, output)
}
