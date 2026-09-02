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
// @Failure      400 {object} nubo_error.PublicErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      500 {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /style [patch]
func UpdateStyleHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input user_settings_models.UpdateStyleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	if err := user_settings_service.UpdateStyle(c.Request.Context(), userID, input); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Style mis à jour avec succès"})
}
