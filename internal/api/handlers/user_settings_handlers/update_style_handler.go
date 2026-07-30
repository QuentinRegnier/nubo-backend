package user_settings_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/user_settings_service"
	"github.com/gin-gonic/gin"
)

// UpdateStyleHandler godoc
// @Summary      Mettre à jour l'apparence
// @Description  Met à jour le thème d'affichage et la langue de l'application.
// @Tags         Réglages
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body user_settings_models.UpdateStyleInput true "Réglages de style à mettre à jour"
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      400 {object} nubo_error.ErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.ErrorResponse "Non autorisé"
// @Failure      500 {object} nubo_error.ErrorResponse "Erreur interne"
// @Router       /style [patch]
func UpdateStyleHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Non autorisé"})
		return
	}

	var input user_settings_models.UpdateStyleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Invalid JSON: " + err.Error()})
		return
	}

	if err := user_settings_service.UpdateStyle(c.Request.Context(), userID, input); err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Impossible de mettre à jour le style"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Style mis à jour avec succès"})
}
