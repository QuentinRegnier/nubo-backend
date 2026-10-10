package member_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/numan-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
	"github.com/QuentinRegnier/numan-backend/internal/pkg"
	"github.com/QuentinRegnier/numan-backend/internal/service/member_service"
	"github.com/gin-gonic/gin"
)

// MuteMemberHandler godoc
// @Summary      Restreindre un membre (Mute)
// @Description  Applique une restriction de parole à un membre dans une conversation pour une durée optionnelle.
// @Description
// @Description  **Authentication & Authorization:**
// @Description  - Authentication requirements: L'utilisateur doit être authentifié avec un token valide.
// @Description  - Required permissions or roles: L'appelant doit être administrateur ou propriétaire et de rang strictement supérieur à la cible.
// @Description
// @Description  **Execution Workflow:**
// @Description  1. Validation du payload (durée, cible).
// @Description  2. Vérification des permissions de l'appelant.
// @Description  3. Vérification de la hiérarchie pour interdire le mute d'un supérieur.
// @Description  4. Mise à jour des permissions du membre en base de données.
// @Description  5. Notification des services en temps réel et invalidation des caches.
// @Description
// @Description  **Error Responses & Reproduction Conditions:**
// @Description
// @Description  🔴 **400 Bad Request:**
// @Description
// @Description  - **[numan_error.CodeInvalidPayload] Requête invalide ou état métier incohérent:**
// @Description    - Trigger: Payload invalide ou ce membre n'a déjà pas le droit de parole en raison des permissions actuelles du groupe.
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
// @Description    - Trigger: L'appelant n'est pas admin, ou tente de muter un membre de rang égal ou supérieur.
// @Description    - Execution stage: Évaluation des règles métier dans le service.
// @Description    - Response: `numan_error.PublicErrorResponse`.
// @Description    - Error code: `numan_error.CodeForbidden`
// @Description
// @Description  🟤 **404 Not Found:**
// @Description
// @Description  - **[numan_error.CodeNotFound] Ressource introuvable:**
// @Description    - Trigger: Conversation introuvable ou inactive.
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
// @Param        input         body   member_models.MuteMemberInput true "Payload pour restreindre un membre dans une conversation"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  numan_error.PublicErrorResponse "Invalid Payload"
// @Failure      401  {object}  numan_error.PublicErrorResponse "Unauthorized"
// @Failure      403  {object}  numan_error.PublicErrorResponse "Forbidden"
// @Failure      404  {object}  numan_error.PublicErrorResponse "Not Found"
// @Failure      500  {object}  numan_error.PublicErrorResponse "Internal Server Error"
// @Router       /conversations/members/mute [post]
func MuteMemberHandler(c *gin.Context) {
	// 1. Extraction propre du Caller via le contexte
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		numan_error.RespondWithError(c, numan_error.NewUnauthorized(numan_error.CodeUserIsNotIdentified, "Utilisateur non identifié.", err))
		return
	}

	// 2. Parsing du JSON (URL clean !)
	var input member_models.MuteMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		numan_error.RespondWithError(c, numan_error.NewBadRequest(numan_error.CodeInvalidPayload, "Payload invalide.", err))
		return
	}

	// 3. Appel du Service
	if err := member_service.MuteMember(c.Request.Context(), callerID, input); err != nil {
		numan_error.RespondWithError(c, err)
		return
	}

	// 4. Succès
	c.JSON(http.StatusOK, gin.H{"message": "Statut de restriction mis à jour."})
}
