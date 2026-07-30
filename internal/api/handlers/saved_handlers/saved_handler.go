package saved_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/saved_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/saved_service"
	"github.com/gin-gonic/gin"
)

// SavePostHandler godoc
// @Summary      Sauvegarder un post
// @Description  Ajoute un post aux favoris (signets) de l'utilisateur.
// @Tags         Favoris
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body saved_models.SaveActionInput true "ID du post à sauvegarder"
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      400 {object} nubo_error.ErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.ErrorResponse "Non autorisé"
// @Failure      404 {object} nubo_error.ErrorResponse "Post introuvable"
// @Router       /saved [post]
func SavePostHandler(c *gin.Context) {
	handleSavedAction(c, "save")
}

// UnsavePostHandler godoc
// @Summary      Retirer un post sauvegardé
// @Description  Enlève un post de la liste des favoris de l'utilisateur.
// @Tags         Favoris
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body saved_models.SaveActionInput true "ID du post à retirer"
// @Success      200 {object} map[string]string "Message de succès"
// @Router       /saved [delete]
func UnsavePostHandler(c *gin.Context) {
	handleSavedAction(c, "unsave")
}

func handleSavedAction(c *gin.Context, action string) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Non autorisé"})
		return
	}

	var input saved_models.SaveActionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Invalid JSON: " + err.Error()})
		return
	}

	if err := saved_service.ToggleSaved(c.Request.Context(), userID, input.PostID, action); err != nil {
		c.JSON(http.StatusNotFound, nubo_error.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Action '" + action + "' exécutée avec succès"})
}
