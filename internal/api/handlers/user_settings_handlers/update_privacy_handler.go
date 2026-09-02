package user_settings_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/user_settings_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/user_settings_service"
	"github.com/gin-gonic/gin"
)

// UpdatePrivacyHandler godoc
// @Summary      Mettre à jour la confidentialité
// @Description  Met à jour partiellement les paramètres de confidentialité de l'utilisateur (PATCH).
// @Tags         Réglages
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        payload body user_settings_models.UpdatePrivacyInput true "Champs de confidentialité à mettre à jour"
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      400 {object} nubo_error.PublicErrorResponse "JSON invalide"
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      500 {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /privacy [patch]
func UpdatePrivacyHandler(c *gin.Context) {
	// 1. Extraction sécurisée de l'ID utilisateur
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 2. Parsage du JSON plat
	var input user_settings_models.UpdatePrivacyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	// 3. Appel du service
	if err := user_settings_service.UpdatePrivacy(c.Request.Context(), userID, input); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	// 4. Succès
	c.JSON(http.StatusOK, gin.H{"message": "Confidentialité mise à jour avec succès"})
}
