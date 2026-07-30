package user_settings_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/user_settings_service"
	"github.com/gin-gonic/gin"
)

// UpdateNotificationsHandler godoc
// @Summary      Mettre à jour les notifications
// @Description  Met à jour partiellement les réglages de notifications push et email de l'utilisateur (PATCH).
// @Tags         Réglages
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body user_settings_models.UpdateNotificationsInput true "Réglages de notifications à mettre à jour"
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      400 {object} nubo_error.ErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.ErrorResponse "Non autorisé"
// @Failure      500 {object} nubo_error.ErrorResponse "Erreur interne"
// @Router       /notifications [patch]
func UpdateNotificationsHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Non autorisé"})
		return
	}

	var input user_settings_models.UpdateNotificationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Invalid JSON: " + err.Error()})
		return
	}

	if err := user_settings_service.UpdateNotifications(c.Request.Context(), userID, input); err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Impossible de mettre à jour les notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notifications mises à jour avec succès"})
}
