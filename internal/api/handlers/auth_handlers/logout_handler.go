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
// @Failure      401 {object} nubo_error.ErrorResponse "Non autorisé"
// @Router       /logout [post]
func LogoutHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Non autorisé"})
		return
	}

	// 2. Extraction du Device Token (Claim "dev" du JWT, placé dans le contexte par ton middleware)
	firebaseInstallationIDs := c.GetString("firebase_installation_id")
	if firebaseInstallationIDs == "" {
		// Fallback de sécurité au cas où la clé dans le contexte s'appelle différemment
		firebaseInstallationIDs = c.GetString("dev")
	}

	// 3. Appel du service de déconnexion
	if err := auth_service.Logout(c.Request.Context(), callerID, firebaseInstallationIDs); err != nil {
		// On ne bloque pas le client sur une erreur de logout, on l'informe juste
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Erreur partielle lors de la déconnexion"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Déconnexion réussie"})
}
