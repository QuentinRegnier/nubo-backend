package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetConversationsHandler godoc
// @Summary      Récupérer un lot de conversations
// @Description  Récupère les métadonnées et aperçus de plusieurs conversations via leurs identifiants (Hydratation par lot L1 -> L2 -> L3).
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description  - Security checks: Filtre uniquement les informations publiques des communautés ou les données accessibles au requérant, expurgeant ce qui est inaccessible.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `conversation_ids` (tableau d'int64, envoyé dans le corps de la requête).
// @Description  - Validation rules: Binding JSON. Rejet si le tableau est vide.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Binding des IDs demandés depuis le JSON.
// @Description  2. Le service `GetConversations` interroge massivement l'Object Cache L1 (Pipeline Redis).
// @Description  3. Les identifiants introuvables (Cache-Miss) font l'objet d'un fallback MongoDB L2 (puis PostgreSQL L3).
// @Description  4. Renflouement (auto-guérison) automatique du Cache L1 avec les données trouvées en base.
// @Description  5. Les données sont agrégées et retournées dans un dictionnaire.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.GetInboxOutput` (Map ID -> ConversationView).
// @Description  - Persistence guarantees: Lecture uniquement (hydratation L1 au besoin).
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description  - **[INVALID_PAYLOAD] Format JSON invalide:**
// @Description    - **Trigger:** Le payload ne correspond pas à la structure attendue.
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
// @Description    - **Trigger:** Erreur inattendue de connexion aux caches ou bases de données.
// @Description    - **Execution stage:** À n'importe quel point du traitement interne, généralement lors d'un appel à un service externe ou une base de données.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeInternalError` (returned in the `code` field of `numan_error.PublicErrorResponse`).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   conversation_models.GetConversationsInput true "Liste des identifiants"
// @Success      200  {object}  conversation_models.GetInboxOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error"
// @Router       /conversation/get [post]
func GetConversationsHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Récupération du joli JSON
	var input conversation_models.GetConversationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou tableau vide.", err))
		return
	}

	// 3. Appel du service
	views, errService := conversation_service.GetConversations(c.Request.Context(), callerID, input)
	if errService != nil {
		numan_error.RespondWithError(c, errService)
		return
	}

	// 4. Renvoi du tableau
	c.JSON(http.StatusOK, views)
}
