package notification_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/notification_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/notification_service"
	"github.com/gin-gonic/gin"
)

// SyncActivityHandler godoc
// @Summary      Synchroniser les notifications (Delta Sync)
// @Description  Vérifie le Delta Sync des activités. Si le client est à jour, retourne need_update=false. Sinon, retourne les notifications manquantes.
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        data          body   notification_models.SyncActivityInput true "État du cache local"
// @Success      200  {object} notification_models.SyncActivityOutput
// @Failure      400  {object} nubo_error.ErrorResponse "Données invalides"
// @Failure      401  {object} nubo_error.ErrorResponse "Non autorisé"
// @Router       /sync/activity [post]
func SyncActivityHandler(c *gin.Context) {
	var input notification_models.SyncActivityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, nubo_error.ErrorResponse{Error: "Format JSON invalide"})
		return
	}

	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, nubo_error.ErrorResponse{Error: "Utilisateur non identifié"})
		return
	}

	output, err := notification_service.SyncActivity(c.Request.Context(), callerID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, nubo_error.ErrorResponse{Error: "Erreur interne lors de la synchronisation"})
		return
	}

	c.JSON(http.StatusOK, output)
}
