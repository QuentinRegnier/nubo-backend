package notification_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/notification_service"
	"github.com/gin-gonic/gin"
)

// ReadNotificationsHandler godoc
// @Summary      Marquer les notifications comme lues
// @Description  Met à jour le curseur de lecture de l'utilisateur (ID Snowflake) et synchronise tous ses appareils via WebSocket.
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        data          body   notification_models.ReadNotificationsInput true "ID maximum lu"
// @Success      200  {object} map[string]string "Statut du succès"
// @Failure      400  {object} nubo_error.ErrorResponse "Données invalides ou JSON malformé"
// @Failure      401  {object} nubo_error.ErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      500  {object} nubo_error.ErrorResponse "Erreur interne"
// @Router       /notifications/read [post]
func ReadNotificationsHandler(c *gin.Context) {
	var input notification_models.ReadNotificationsInput

	// Ton middleware s'attend souvent à lire le body "data" si c'est du multipart,
	// mais pour un JSON pur (comme utilisé dans CreateConversationHandler), ShouldBindJSON est parfait.
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format JSON invalide ou champs manquants"})
		return
	}

	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	if err := notification_service.MarkNotificationsAsRead(c.Request.Context(), callerID, input); err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Erreur interne lors de la mise à jour du curseur"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
