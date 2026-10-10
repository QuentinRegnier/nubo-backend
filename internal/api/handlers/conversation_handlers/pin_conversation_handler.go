package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// PinConversationHandler godoc
// @Summary      Épingler une conversation
// @Description  Épingle une conversation en haut de l'inbox de l'utilisateur.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description  - Security checks: Vérifie que l'utilisateur n'a pas dépassé le plafond d'épinglages autorisés (ex: max 3).
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `conversation_id`.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Binding de la requête.
// @Description  2. Validation du plafond des épinglages via Cache L1 pour empêcher les abus (bloqué à 3 max).
// @Description  3. Ajout de la conversation au set des favoris (Cache synchrone).
// @Description  4. Persistance asynchrone (Redis Write-Behind).
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.PinConversationOutput`.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Plafond atteint ou payload invalide:**
// @Description    - **Trigger:** L'utilisateur essaie d'épingler plus de 3 conversations.
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
// @Param        input         body   conversation_models.PinConversationInput true "Payload"
// @Success      200  {object}  conversation_models.PinConversationOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error"
// @Router       /conversation/pin [post]
func PinConversationHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input conversation_models.PinConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou conversation_id manquant.", err))
		return
	}

	output, err := conversation_service.TogglePinConversation(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
