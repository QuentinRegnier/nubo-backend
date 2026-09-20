package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// GetBannedMembersHandler godoc
// @Summary Liste des utilisateurs bannis
// @Description Récupère la liste paginée des membres bannis d'une conversation via une requête POST.
// @Tags conversations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param request body conversation_models.GetBannedMembersInput true "Données de pagination et ID de la conversation"
// @Success 200 {object} conversation_models.GetBannedMembersOutput
// @Router /conversations/banned [post]
func GetBannedMembersHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input conversation_models.GetBannedMembersInput
	// Le binding récupère directement le payload depuis le body JSON
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_JSON", "Corps de requête invalide.", err))
		return
	}

	// Gestion de la valeur par défaut pour le JSON
	if input.Limit == 0 {
		input.Limit = 50
	}

	output, errOut := conversation_service.GetBannedMembers(c.Request.Context(), callerID, input)
	if errOut != nil {
		nubo_error.RespondWithError(c, errOut)
		return
	}

	c.JSON(http.StatusOK, output)
}
