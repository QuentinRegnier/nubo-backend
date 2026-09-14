package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// AcceptCommunityRequestHandler godoc
// @Summary Accepter une demande d'adhésion
// @Description Permet à un administrateur d'accepter un utilisateur en attente (Role = -3) dans la communauté.
// @Description Le membre passe au statut actif, un message système est publié et un événement WebSocket est diffusé.
// @Tags conversations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param input body conversation_models.AcceptCommunityRequestInput true "Identifiants de la conversation et de la cible"
// @Success 200 {object} conversation_models.AcceptCommunityRequestOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 401 {object} nubo_error.PublicErrorResponse
// @Failure 403 {object} nubo_error.PublicErrorResponse
// @Failure 404 {object} nubo_error.PublicErrorResponse
// @Router /conversations/communities/decision/accept [post]
func AcceptCommunityRequestHandler(c *gin.Context) {
	// 1. Récupération du CallerID via le bon package
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Récupération et parsing du JSON
	var input conversation_models.AcceptCommunityRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Corps de requête invalide.", err))
		return
	}

	// 3. Appel du service métier
	output, err := conversation_service.AcceptCommunityRequest(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Réponse
	c.JSON(http.StatusOK, output)
}
