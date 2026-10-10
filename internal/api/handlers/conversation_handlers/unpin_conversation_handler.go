package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// UnpinConversationHandler godoc
// @Summary      Désépingler une conversation
// @Description  Retire une conversation de la liste prioritaire (épinglée) de l'inbox utilisateur.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `conversation_id`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Binding de la requête JSON.
// @Description  2. Retrait immédiat de l'identifiant dans l'Object Cache (L1) et le ZSET des épingles.
// @Description  3. Délégation asynchrone pour propager la désélection dans MongoDB / PostgreSQL (Write-Behind).
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.UnpinConversationOutput`.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Paramètres invalides:**
// @Description    - **Trigger:** Le payload JSON de la requête est absent, mal formaté ou ne respecte pas les contraintes de validation du modèle attendu.
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
// @Param        input         body   conversation_models.UnpinConversationInput true "Identifiant de la conversation à désépingler"
// @Success      200  {object}  conversation_models.UnpinConversationOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error"
// @Router       /conversation/unpin [delete]
func UnpinConversationHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input conversation_models.UnpinConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou conversation_id manquant.", err))
		return
	}

	output, err := conversation_service.UnpinConversation(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
