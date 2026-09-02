package auth_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/auth_service"
	"github.com/gin-gonic/gin"
)

// GetSessionsHandler godoc
// @Summary      Lister les sessions actives
// @Description  Renvoie la liste des appareils et sessions connectés à ce compte.
// @Description  Les données renvoyées sont strictement expurgées (aucun secret, aucun token).
// @Tags         Sécurité
// @Produce      json
// @Security     ApiKeyAuth
// @Success      200 {array}  auth_models.SessionView
// @Failure      401 {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Failure      500 {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /sessions [get]
func GetSessionsHandler(c *gin.Context) {
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	sessions, err := auth_service.GetUserSessions(c.Request.Context(), userID)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, sessions)
}
