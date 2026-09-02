package auth_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// LogoutHandler godoc
// @Summary      Déconnexion (Logout)
// @Description  Déconnecte l'appareil actuel en révoquant la session associée au JWT appelant.
// @Tags         Sécurité
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Router       /logout [post]
func LogoutHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	firebaseInstallationIDs := c.GetString("firebase_installation_id")
	if firebaseInstallationIDs == "" {
		firebaseInstallationIDs = c.GetString("dev")
	}

	if err := auth_service.Logout(c.Request.Context(), callerID, firebaseInstallationIDs); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Déconnexion réussie"})
}
