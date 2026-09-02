package notification_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/notification_service"
	"github.com/gin-gonic/gin"
)

// GetNotificationsHandler godoc
// @Summary      Récupérer les notifications
// @Description  Récupère la liste paginée des notifications de l'utilisateur. Utiliser force=true pour forcer la mise à jour depuis la base de données.
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        data          body   notification_models.GetNotificationsInput true "Paramètres de pagination"
// @Success      200  {object} notification_models.GetNotificationsOutput
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données invalides ou JSON malformé"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Session expirée ou utilisateur non identifié"
// @Failure      500  {object} nubo_error.PublicErrorResponse "Erreur interne"
// @Router       /notifications/get [post]
func GetNotificationsHandler(c *gin.Context) {
	var input notification_models.GetNotificationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide ou paramètres manquants.", err))
		return
	}

	// 🛡️ BOUCLIER DE PAGINATION
	if input.Limit <= 0 || input.Limit > 100 {
		input.Limit = 50
	}

	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	notifs, err := notification_service.GetNotifications(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	if notifs == nil {
		notifs = make([]notification_models.NotificationPayload, 0)
	}

	c.JSON(http.StatusOK, notification_models.GetNotificationsOutput{Notifications: notifs})
}
