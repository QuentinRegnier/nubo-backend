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
// @Failure      400 {object} nubo_error.PublicErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      404 {object} nubo_error.PublicErrorResponse "Post introuvable"
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
		nubo_error.RespondWithError(c, err)
		return
	}

	var input saved_models.SaveActionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	if err := saved_service.ToggleSaved(c.Request.Context(), userID, input.PostID, action); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Action '" + action + "' exécutée avec succès"})
}
