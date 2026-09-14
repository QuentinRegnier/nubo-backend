package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// SuggestContactsHandler gère la route POST /conversation/suggest
func SuggestContactsHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.SuggestInput
	// Binding strict sur le JSON
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Le JSON de la requête est mal formaté.", err))
		return
	}

	// Application manuelle de la valeur par défaut pour la pagination JSON
	if input.Limit == 0 {
		input.Limit = 20
	}

	output, errService := conversation_service.SuggestContacts(c.Request.Context(), callerID, input)
	if errService != nil {
		nubo_error.RespondWithError(c, errService)
		return
	}

	c.JSON(http.StatusOK, output)
}
