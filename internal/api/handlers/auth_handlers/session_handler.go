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
// @Failure      401 {object} nubo_error.ErrorResponse "Non autorisé"
// @Failure      500 {object} nubo_error.ErrorResponse "Erreur interne"
// @Router       /sessions [get]
func GetSessionsHandler(c *gin.Context) {
	// 1. Extraction sécurisée de l'ID utilisateur
	userID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Non autorisé"})
		return
	}

	// 2. Appel du service métier
	sessions, err := auth_service.GetUserSessions(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Impossible de récupérer les sessions"})
		return
	}

	// 3. Retour de la liste expurgée
	c.JSON(http.StatusOK, sessions)
}
