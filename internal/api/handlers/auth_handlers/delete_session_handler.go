package auth_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
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
// @Failure      400 {object} nubo_error.PublicErrorResponse "ID manquant ou invalide"
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      403 {object} nubo_error.PublicErrorResponse "Accès refusé"
// @Router       /sessions [delete]
func DeleteSessionHandler(c *gin.Context) {
	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	var input auth_models.DeleteSessionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou session_id manquant.", err))
		return
	}

	if err := auth_service.RevokeSession(c.Request.Context(), callerID, input.SessionID); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session révoquée avec succès"})
}
