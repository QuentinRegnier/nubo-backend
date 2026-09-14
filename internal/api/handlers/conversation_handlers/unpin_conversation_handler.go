package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// UnpinConversationHandler godoc
// @Summary      Désépingler une conversation
// @Description  Retire l'épingle d'une conversation (remet la valeur à -1).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body conversation_models.UnpinConversationInput true "ID de la conversation"
// @Success      200 {object} conversation_models.UnpinConversationOutput
// @Failure      400 {object} nubo_error.PublicErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      403 {object} nubo_error.PublicErrorResponse "Accès refusé"
// @Router       /conversation/pin [delete]
func UnpinConversationHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.UnpinConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou conversation_id manquant.", err))
		return
	}

	output, err := conversation_service.UnpinConversation(c.Request.Context(), userID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
