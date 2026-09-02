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
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données invalides ou JSON malformé"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      500  {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /notifications/read [post]
func ReadNotificationsHandler(c *gin.Context) {
	var input notification_models.ReadNotificationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	if err := notification_service.MarkNotificationsAsRead(c.Request.Context(), callerID, input); err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
