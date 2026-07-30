package saved_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/saved_service"
	"github.com/gin-gonic/gin"
)

// GetSavedPostsHandler godoc
// @Summary      Lister les favoris
// @Description  Récupère la liste paginée des posts sauvegardés par l'utilisateur.
// @Tags         Favoris
// @Produce      json
// @Security     ApiKeyAuth
// @Param        limit  query int false "Nombre max de posts (défaut: 50)"
// @Param        offset query int false "Décalage pour la pagination (défaut: 0)"
// @Success      200 {array}  post_models.GetPostOutput
// @Failure      400 {object} nubo_error.ErrorResponse "Paramètres invalides"
// @Failure      401 {object} nubo_error.ErrorResponse "Non autorisé"
// @Router       /saved [get]
func GetSavedPostsHandler(c *gin.Context) {
	// 1. Extraction sécurisée de l'appelant
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Non autorisé"})
		return
	}

	// 2. Récupération des paramètres Query String
	var input saved_models.GetSavedInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Invalid query parameters: " + err.Error()})
		return
	}

	// 3. Appel au service métier (Cascade d'IDs puis Pipeline GetPosts)
	results := saved_service.GetSavedPosts(c.Request.Context(), userID, input.Limit, input.Offset)

	// 4. Succès
	c.JSON(http.StatusOK, results)
}
