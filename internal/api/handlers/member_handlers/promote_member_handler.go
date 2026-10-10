package member_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/member_service"
	"github.com/gin-gonic/gin"
)

// PromoteMemberHandler godoc
// @Summary      Promouvoir un membre en administrateur
// @Description  Élève le rang d'un membre normal au statut d'administrateur dans la conversation.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: L'utilisateur doit être authentifié avec un token valide.
// @Description  - Required permissions or roles: L'utilisateur appelant doit être le propriétaire (Owner) de la conversation.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Validation GIN du payload.
// @Description  2. Vérification que l'appelant est le propriétaire.
// @Description  3. Vérification que la cible est un membre actif de la conversation.
// @Description  4. Mise à jour du rôle en base de données.
// @Description  5. Invalidation des caches et propagation des événements de permissions.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[numan_error.CodeInvalidPayload] Requête invalide ou état métier incohérent:**
// @Description    - Trigger: Payload invalide ou la cible n'est pas un membre actif de ce groupe.
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
// @Description    - Trigger: Seul le propriétaire actuel peut promouvoir un membre.
// @Description    - Execution stage: Évaluation des règles métier dans le service.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden`
// @Description
// @Description  🟤 **404 Not Found:**
// @Description
// @Description  - **[numan_error.CodeNotFound] Ressource introuvable:**
// @Description    - Trigger: Conversation ou membre introuvable.
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
// @Param        input         body   member_models.PromoteMemberInput true "Payload pour promouvoir un membre dans un groupe"
// @Success      200  {object}  member_models.PromoteMemberOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /group/promote/set [post]
func PromoteMemberHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	var input member_models.PromoteMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	output, err := member_service.PromoteMember(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}
