package auth_handlers

import (
	"net/http"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// DeleteSessionHandler godoc
// @Summary      Révoquer une session
// @Description  Déconnecte un appareil à distance.
// @Tags         Sécurité
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id query string true "ID de la session à révoquer"
// @Success      200 {object} map[string]string "Message de succès"
// @Failure      400 {object} nubo_error.ErrorResponse "ID manquant ou invalide"
// @Failure      401 {object} nubo_error.ErrorResponse "Non autorisé"
// @Failure      403 {object} nubo_error.ErrorResponse "Accès refusé"
// @Router       /sessions [delete]
func DeleteSessionHandler(c *gin.Context) {
	// 1. Identification de l'appelant
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Non autorisé"})
		return
	}

	// 2. Extraction via Query String (pas de /:id)
	sessionIDStr := c.Query("id")
	if sessionIDStr == "" {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Le paramètre 'id' est requis"})
		return
	}

	sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format d'ID invalide"})
		return
	}

	// 3. Appel du service de révocation
	if err := auth_service.RevokeSession(c.Request.Context(), callerID, sessionID); err != nil {
		c.JSON(http.StatusForbidden, nubo_error.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session révoquée avec succès"})
}
