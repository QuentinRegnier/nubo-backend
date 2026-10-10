package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// UpdateConversationHandler godoc
// @Summary      Modifier une conversation ou ses métadonnées
// @Description  Met à jour les paramètres, métadonnées ou le titre d'une conversation de groupe ou d'une communauté publique.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: Route sécurisée (Token JWT requis).
// @Description  - Security checks: Valide le rôle de l'utilisateur (Administrateur requis selon l'attribut visé). Verrouille la modification si la conversation est un DM.
// @Description
// @Description  **Request Contract:**
// @Description  - Required fields: `conversation_id`.
// @Description  - Optional fields: Titre, description, avatar_id, lien, paramètres d'ajout (selon conversation_models.UpdateConversationInput).
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Binding GIN de la structure JSON de mise à jour.
// @Description  2. Chargement de l'enveloppe de conversation via le Cache L1/L2.
// @Description  3. Contrôles métiers stricts :
// @Description     - Rejet direct avec 403 Forbidden si l'utilisateur tente d'éditer un Message Privé (Type 1).
// @Description     - Rejet (403) s'il tente de définir une `Description`, un `Avatar` ou un `Lien externe` sur un groupe privé classique (ces attributs sont restreints aux communautés publiques).
// @Description  4. Mutation en mémoire et écrasement synchrone dans le Speed Cache L1.
// @Description  5. Envoi dans la file Write-Behind (`EntityConversation`, `ActionUpdate`) pour synchroniser Mongo/Postgres.
// @Description  6. Diffusion WebSocket pour alerter les participants en temps réel des changements structurels du groupe.
// @Description
// @Description  **Success Behavior:**
// @Description  - HTTP status: 200 OK
// @Description  - Response body: `conversation_models.UpdateConversationOutput`.
// @Description  - Persistence guarantees: L1 synchrone, durabilité L2/L3 asynchrone, diffusion Realtime instantanée.
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
// @Description  🔴 **403 Forbidden:**
// @Description  - **[FORBIDDEN_ACCESS] Modification impossible ou non autorisée:**
// @Description    - **Trigger:** La cible est un Message Privé, ou l'utilisateur altère un groupe classique avec des attributs réservés aux communautés publiques (Type 3).
// @Description    - **Execution stage:** Validation des règles métier.
// @Description    - **Response:** `numan_error.PublicErrorResponse` contenant le message d'erreur.
// @Description    - **Error code:** `numan_error.CodeForbidden` (returned in the `code` field of `numan_error.PublicErrorResponse`).
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
// @Param        input         body   conversation_models.UpdateConversationInput true "Payload de modification de la conversation"
// @Success      200  {object}  conversation_models.UpdateConversationOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Error"
// @Router       /conversation/update [put]
func UpdateConversationHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input conversation_models.UpdateConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := conversation_service.UpdateConversation(c.Request.Context(), callerID, input.ConversationID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}
