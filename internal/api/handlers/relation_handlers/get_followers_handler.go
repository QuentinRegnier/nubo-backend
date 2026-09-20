package relation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/relation_service"
	"github.com/gin-gonic/gin"
)

// GetFollowersHandler godoc
// @Summary Liste des abonnés
// @Description Récupère la liste paginée des abonnés d'un utilisateur.
// @Tags relations
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param request body relation_models.GetFollowersInput true "Données de pagination"
// @Success 200 {object} relation_models.GetFollowersOutput
// @Router /relations/followers [post]
func GetFollowersHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input relation_models.GetFollowersInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_JSON", "Corps de requête invalide.", err))
		return
	}
	if input.Limit == 0 {
		input.Limit = 50
	}

	// Cible à remplacer par la vraie variable dans ton scope
	output, errOut := relation_service.GetFollows(c.Request.Context(), callerID, input)
	if errOut != nil {
		nubo_error.RespondWithError(c, errOut)
		return
	}

	c.JSON(http.StatusOK, output)
}
