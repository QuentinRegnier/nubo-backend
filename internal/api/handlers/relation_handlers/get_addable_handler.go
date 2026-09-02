package relation_handlers

import (
	"net/http"
	"strings"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/relation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/relation_service"
	"github.com/gin-gonic/gin"
)

// GetAddableHandler godoc
// @Summary Suggestions d'ajout au groupe
// @Description Récupère de manière paginée la liste des amis et abonnements de l'utilisateur, triée par niveau d'affinité puis alphabétiquement.
// @Tags groups
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <votre_jwt>"
// @Param X-Signature header string true "Signature HMAC de la requête"
// @Param X-Timestamp header string true "Timestamp Unix de la requête"
// @Param limit query int false "Limite (défaut 50, max 100)"
// @Param offset query int false "Décalage pour pagination (défaut 0)"
// @Param force query bool false "Forcer le rafraîchissement du cache"
// @Success 200 {object} conversation_models.GetAddableOutput
// @Failure 400 {object} nubo_error.PublicErrorResponse "Paramètres invalides"
// @Failure 401 {object} nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure 500 {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router /group/addable [get]
// @Router /group/addable/force [get]
func GetAddableHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input relation_models.GetAddableInput
	if err := c.ShouldBindQuery(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_QUERY", "Paramètres de pagination invalides.", err))
		return
	}

	// 🛡️ BOUCLIER DE PAGINATION
	if input.Limit <= 0 || input.Limit > 100 {
		input.Limit = 50
	}

	if strings.HasSuffix(c.Request.URL.Path, "/force") {
		input.Force = true
	}

	output, err := relation_service.GetAddableUsers(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
