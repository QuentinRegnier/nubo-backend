package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetMutedMembersHandler godoc
// @Summary Lister les membres restreints
// @Description Récupère la liste des utilisateurs mutés avec leur timestamp de fin. JSON uniquement. (Admin/Propriétaire requis)
// @Tags conversations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param payload body conversation_models.GetMutedMembersInput true "Paramètres de recherche"
// @Success 200 {object} conversation_models.GetMutedMembersOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse
// @Failure 403 {object} nubo_error.PublicErrorResponse
// @Router /conversations/members/muted [post]
func GetMutedMembersHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.GetMutedMembersInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_JSON", "Payload invalide.", err))
		return
	}

	output, err := conversation_service.GetMutedMembers(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
