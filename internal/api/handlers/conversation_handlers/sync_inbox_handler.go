package conversation_handlers

import (
	"net/http"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
	"github.com/QuentinRegnier/nubo-backend/internal/service/conversation_service"
	"github.com/gin-gonic/gin"
)

// SyncInboxHandler godoc
// @Summary      Synchroniser la boîte de réception
// @Description  Vérifie le Delta Sync. Si le client est à jour, retourne HTTP 200 avec need_update=false. Sinon, hydrate et retourne l'inbox complète.
// @Tags         conversations
// @Accept       json
// @Produce      json
// @Param        Authorization header string true  "Bearer <votre_jwt>"
// @Param        X-Signature   header string true  "Signature HMAC de la requête"
// @Param        X-Timestamp   header string true  "Timestamp Unix de la requête"
// @Param        data          body   conversation_models.SyncInboxInput true "Date de dernière mise à jour locale"
// @Success      200  {object} conversation_models.SyncInboxOutput
// @Failure      400  {object} nubo_error.PublicErrorResponse "Données invalides"
// @Failure      401  {object} nubo_error.PublicErrorResponse "Non autorisé"
// @Router       /sync/inbox [post]
func SyncInboxHandler(c *gin.Context) {
	var input conversation_models.SyncInboxInput
	if err := c.ShouldBindJSON(&input); err != nil {
		nubo_error.RespondWithError(c, nubo_error.NewBadRequest("INVALID_PAYLOAD", "Format JSON invalide.", err))
		return
	}

	callerID, err := pkg.GetUserIDFromContext(c)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	output, err := conversation_service.SyncInbox(c.Request.Context(), callerID, input)
	if err != nil {
		nubo_error.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
