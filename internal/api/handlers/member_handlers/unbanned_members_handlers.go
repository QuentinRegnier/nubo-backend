package member_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/member_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/member_service"
	"github.com/gin-gonic/gin"
)

// UnbanMembersHandler godoc
// @Summary Débannir un lot d'utilisateurs
// @Description Permet à un administrateur de débannir un ou plusieurs utilisateurs d'une conversation.
// @Tags conversations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param request body conversation_models.UnbanMembersInput true "Données d'entrée (conversation_id et target_user_ids)"
// @Success 200 {object} conversation_models.UnbanMembersOutput
// @Router /conversations/unban [post]
func UnbanMembersHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input member_models.UnbanMembersInput
	// Le binding récupère directement le conversation_id depuis le body JSON
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_JSON", "Corps de requête invalide.", err))
		return
	}

	output, errOut := member_service.UnbanMembers(c.Request.Context(), callerID, input)
	if errOut != nil {
		nubo_error.RespondWithError(c, errOut)
		return
	}

	c.JSON(http.StatusOK, output)
}
