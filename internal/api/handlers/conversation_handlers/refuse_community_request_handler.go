package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// RefuseCommunityRequestHandler godoc
// @Summary Refuser une demande d'adhésion
// @Description Permet à un administrateur de rejeter un utilisateur en attente (Role = -3).
// @Description La demande est supprimée (Hard Delete), permettant à l'utilisateur de postuler de nouveau plus tard s'il le souhaite.
// @Tags conversations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param input body conversation_models.RefuseCommunityRequestInput true "Identifiants de la conversation et de la cible"
// @Success 200 {object} conversation_models.RefuseCommunityRequestOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 401 {object} nubo_error.PublicErrorResponse
// @Failure 403 {object} nubo_error.PublicErrorResponse
// @Failure 404 {object} nubo_error.PublicErrorResponse
// @Router /conversations/communities/decision/refusal [post]
func RefuseCommunityRequestHandler(c *gin.Context) {
	// 1. Récupération du CallerID via le bon package
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Récupération et parsing du JSON
	var input conversation_models.RefuseCommunityRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Corps de requête invalide.", err))
		return
	}

	// 3. Appel du service métier
	output, err := conversation_service.RefuseCommunityRequest(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Réponse
	c.JSON(http.StatusOK, output)
}
