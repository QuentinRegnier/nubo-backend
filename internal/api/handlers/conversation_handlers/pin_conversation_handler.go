package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// PinConversationHandler godoc
// @Summary      Épingler / Désépingler
// @Description  Épingle ou désépingle une conversation (Maximum 3 épingles par utilisateur).
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body conversation_models.PinConversationInput true "ID de la conversation"
// @Success      200 {object} conversation_models.PinConversationOutput
// @Router       /conversation/pin [post]
func PinConversationHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.PinConversationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide (action 'pin' ou 'unpin' requise).", err))
		return
	}

	output, err := conversation_service.TogglePinConversation(c.Request.Context(), userID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
