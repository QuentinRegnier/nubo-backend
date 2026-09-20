package relation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/relation_service"
	"github.com/gin-gonic/gin"
)

// GetBlockedUsersHandler godoc
// @Summary Liste des utilisateurs bloqués
// @Description Récupère la liste paginée des utilisateurs bloqués par l'appelant.
// @Tags relations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param request body relation_models.GetBlockedInput true "Données de pagination"
// @Success 200 {object} relation_models.GetBlockedOutput
// @Router /relations/blocked [post]
func GetBlockedUsersHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input relation_models.GetBlockedInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_JSON", "Corps de requête invalide.", err))
		return
	}
	if input.Limit == 0 {
		input.Limit = 50
	}

	output, errOut := relation_service.GetBlockedUsers(c.Request.Context(), callerID, input)
	if errOut != nil {
		nubo_error.RespondWithError(c, errOut)
		return
	}

	c.JSON(http.StatusOK, output)
}
