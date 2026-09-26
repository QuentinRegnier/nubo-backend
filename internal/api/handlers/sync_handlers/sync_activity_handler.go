package sync_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/sync_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/sync_service"
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
// @Param        data          body   sync_models.SyncActivityInput true "État du cache local"
// @Success      200  {object} notification_models.SyncActivityOutput
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données invalides"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Router       /sync/activity [post]
func SyncActivityHandler(c *gin.Context) {
	var input sync_models.SyncActivityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide.", err))
		return
	}

	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	output, err := sync_service.SyncActivity(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
