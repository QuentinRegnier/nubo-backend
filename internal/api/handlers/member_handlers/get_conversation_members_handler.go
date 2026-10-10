package member_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetConversationMembersHandler godoc
// @Summary      Lister les membres d'une conversation
// @Description  Récupère la liste des membres actifs d'une conversation.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: L'utilisateur doit être authentifié avec un token valide.
// @Description  - Required permissions or roles: L'utilisateur appelant doit être membre actif de la conversation.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Validation des paramètres (limite, offset).
// @Description  2. Vérification que l'appelant fait partie de la conversation.
// @Description  3. Récupération de la liste des membres depuis la base de données ou cache.
// @Description  4. Construction des vues de profil et retour au client.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[numan_error.CodeInvalidPayload] Requête invalide ou état métier incohérent:**
// @Description    - Trigger: Paramètres de requête invalides.
// @Description    - Execution stage: Validation GIN ou vérification des règles métier dans le service.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInvalidPayload`
// @Description
// @Description  🟡 **401 Unauthorized:**
// @Description
// @Description  - **[numan_error.CodeUnauthorized] Utilisateur non authentifié:**
// @Description    - Trigger: Le token d'authentification est manquant, invalide ou expiré.
// @Description    - Execution stage: Middleware d'authentification.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeUnauthorized`
// @Description
// @Description  🟠 **403 Forbidden:**
// @Description
// @Description  - **[numan_error.CodeForbidden] Permissions insuffisantes:**
// @Description    - Trigger: L'appelant n'a pas accès à cette conversation.
// @Description    - Execution stage: Évaluation des règles métier dans le service.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden`
// @Description
// @Description  🟤 **404 Not Found:**
// @Description
// @Description  - **[numan_error.CodeNotFound] Ressource introuvable:**
// @Description    - Trigger: Conversation introuvable.
// @Description    - Execution stage: Requête en base de données.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeNotFound`
// @Description
// @Description  ⚫ **500 Internal Server Error:**
// @Description
// @Description  - **[numan_error.CodeInternalError] Erreur serveur inattendue:**
// @Description    - Trigger: Panne base de données, timeout, ou erreur de service inattendue.
// @Description    - Execution stage: Traitement des données.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeInternalError`
// @Tags         members
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer <votre_jwt>"
// @Param        X-Signature   header string true "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true "Timestamp Unix de la requête"
// @Param        input         body   member_models.GetConversationMembersInput true "Payload pour récupérer la liste des membres d'une conversation"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /conversation/members/get [post]
func GetConversationMembersHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input member_models.GetConversationMembersInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou tableau vide.", err))
		return
	}

	views, errService := conversation_service.GetConversationMembers(c.Request.Context(), callerID, input)
	if errService != nil {
		numan_error.RespondWithError(c, errService)
		return
	}

	c.JSON(http.StatusOK, views)
}
