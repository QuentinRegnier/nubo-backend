package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetUserConversationsHandler godoc
// @Summary      Récupérer l'inbox (conversations) de l'utilisateur
// @Description  Renvoie la liste des conversations (Inbox) dans lesquelles l'utilisateur est impliqué, triées par date de dernier message (Dirty ZSET).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT obligatoire).
// @Description  - L'accès est strictement personnel : le système utilise le `callerID` pour charger la boîte de réception.
// @Description
// @Description  **Request Contract:**
// @Description  - Optional parameters: `offset` et `limit` (Query parameters) pour la pagination.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Le service interroge le ZSET `UserConversations` (Cache L1) de l'utilisateur pour extraire les IDs de ses conversations actives triées temporellement.
// @Description  2. En cas de Cache-Miss L1, requête PostgreSQL (L3) pour reconstituer l'inbox en mémoire.
// @Description  3. Les identifiants sont hydratés (métadonnées du groupe, dernier message affiché, aperçus, pastilles non lues).
// @Description  4. Construction et envoi du modèle agrégé de l'inbox.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.GetUserInboxOutput` avec pagination et état `HasMore`.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Paramètres invalides:**
// @Description    - **Trigger:** L'offset ou limit ont un format malformé.
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
// @Description  - **[INTERNAL_SERVER_ERROR] Échec L3:**
// @Description    - **Trigger:** Reconstitution SQL de l'inbox a échoué.
// @Description    - **Execution stage:** À n'importe quel point du traitement interne, généralement lors d'un appel à un service externe ou une base de données.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   conversation_models.GetUserConversationsInput true "Payload de récupération"
// @Success      200  {object}  conversation_models.GetUserInboxOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error"
// @Router       /conversation/user/get [post]
func GetUserConversationsHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input conversation_models.GetUserConversationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Le JSON de la requête est mal formaté.", err))
		return
	}

	output, errService := conversation_service.GetUserConversationsPaginated(c.Request.Context(), callerID, input)
	if errService != nil {
		numan_error.RespondWithError(c, errService)
		return
	}

	c.JSON(http.StatusOK, output)
}
