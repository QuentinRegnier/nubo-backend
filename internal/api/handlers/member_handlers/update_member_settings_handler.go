package member_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/member_service"
	"github.com/gin-gonic/gin"
)

// UpdateMemberSettingsHandler godoc
// @Summary      Mettre à jour les paramètres du membre
// @Description  Modifie les paramètres personnels d'un membre (ex. notifications, surnom) dans le contexte d'une conversation.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: L'utilisateur doit être authentifié avec un token valide.
// @Description  - Required permissions or roles: L'utilisateur appelant doit être le propriétaire des paramètres.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Validation du payload des paramètres.
// @Description  2. Vérification de l'appartenance à la conversation.
// @Description  3. Mise à jour du document de paramètres en base de données.
// @Description  4. Synchronisation avec les caches de profil local.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[numan_error.CodeInvalidPayload] Requête invalide ou état métier incohérent:**
// @Description    - Trigger: Payload de paramètres invalide.
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
// @Description    - Trigger: L'appelant tente de modifier les paramètres d'un autre utilisateur.
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
// @Param        input         body   member_models.UpdateMemberSettingsInput true "Payload pour mettre à jour les paramètres du membre"
// @Success      200  {object}  member_models.UpdateMemberSettingsOutput
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /conversation/settings [patch]
func UpdateMemberSettingsHandler(c *gin.Context) {
	// 1. Récupération de l'identité
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsage et validation du JSON
	var input member_models.UpdateMemberSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Format JSON invalide ou conversation_id manquant.", err))
		return
	}

	// 3. Appel du service métier
	output, err := member_service.UpdateMemberSettings(c.Request.Context(), callerID, input)
	if err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	// 4. Succès direct, pas besoin d'un DTO de sortie
	c.JSON(http.StatusOK, output)
}
